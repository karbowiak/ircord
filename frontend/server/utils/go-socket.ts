import WebSocket from 'ws'
import type { Peer } from 'crossws'

/** One upstream socket per browser socket; authorization remains entirely in Go. */
export class GoSocket {
  private peer?: Peer
  private early: string[] = []
  private earlyBytes = 0
  private readonly socket: WebSocket
  private readonly bindTimer: ReturnType<typeof setTimeout>
  readonly ready: Promise<void>
  constructor(backend: string, request: { url: string; headers: Headers }) {
    const url = new URL(
      backend.replace(/\/$/, '') + new URL(request.url).pathname + new URL(request.url).search,
    )
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
    this.socket = new WebSocket(url, {
      headers: {
        Cookie: request.headers.get('cookie') ?? '',
        Origin: request.headers.get('origin') ?? '',
      },
      handshakeTimeout: 5000,
      maxPayload: 1 << 20,
    })
    this.bindTimer = setTimeout(() => this.close(), 10_000)
    this.bindTimer.unref()
    this.ready = new Promise((resolve, reject) => {
      this.socket.once('open', resolve)
      this.socket.once('unexpected-response', (_request, response) => {
        response.resume()
        this.close()
        reject(
          new Response('WebSocket authorization failed', { status: response.statusCode ?? 502 }),
        )
      })
      this.socket.once('error', () => reject(new Response('Backend unavailable', { status: 503 })))
    })
    this.socket.on('message', (data) => {
      const text = Array.isArray(data)
        ? Buffer.concat(data).toString('utf8')
        : Buffer.isBuffer(data)
          ? data.toString('utf8')
          : Buffer.from(data).toString('utf8')
      if (this.peer) {
        if ((this.peer.websocket.bufferedAmount ?? 0) > 1 << 20) {
          this.peer.close(1013, 'Client fell behind')
          this.close()
          return
        }
        this.peer.send(text)
      } else if (this.early.length < 512 && this.earlyBytes + Buffer.byteLength(text) <= 1 << 20) {
        this.early.push(text)
        this.earlyBytes += Buffer.byteLength(text)
      } else this.close()
    })
    this.socket.on('close', (code, reason) => {
      clearTimeout(this.bindTimer)
      this.peer?.close(
        [1004, 1005, 1006, 1015].includes(code) ? 1011 : code,
        reason.toString().slice(0, 100),
      )
    })
    this.socket.on('error', () => {
      this.peer?.close(1011, 'Backend connection failed')
      this.close()
    })
  }
  bind(peer: Peer) {
    clearTimeout(this.bindTimer)
    this.peer = peer
    if (this.socket.readyState !== WebSocket.OPEN) {
      peer.close(1011, 'Backend connection closed')
      return
    }
    for (const message of this.early) peer.send(message)
    this.early = []
    this.earlyBytes = 0
  }
  send(message: string) {
    if (Buffer.byteLength(message) > 32768 || this.socket.bufferedAmount > 65536) {
      this.peer?.close(1008, 'Invalid event frame')
      this.close()
      return
    }
    if (this.socket.readyState === WebSocket.OPEN) this.socket.send(message)
  }
  close() {
    clearTimeout(this.bindTimer)
    this.early = []
    this.earlyBytes = 0
    this.socket.terminate()
  }
}

export function backendSocketURL() {
  const config = useRuntimeConfig()
  return config.demo ? '' : config.backendUrl.replace(/\/$/, '')
}

/** Pin the external origin when TLS terminates at a reverse proxy. */
export function socketOrigin(requestURL: string) {
  const configured = useRuntimeConfig().siteUrl
  return new URL(configured || requestURL).origin
}
