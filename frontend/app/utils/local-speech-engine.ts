/**
 * IRCord's own client for the vendored ZONOS3-Mini workers (#204), in place of upstream's
 * engine.js: that file starts its workers on import and can never stop them, while read-aloud
 * must free about 1.5 GB of graphics memory when it is switched off. This speaks the workers'
 * message protocol (upstream src/worker.ts InMsg/OutMsg); scripts/vendor-local-speech.mjs pins the
 * worker files, and moving the pin means checking this protocol again.
 */
import { localSpeechBase, localSpeechModelsPath } from '../../shared/local-speech'

/** Bytes of the shards read so far (from the network or the browser's cache), of `total`. */
export interface LocalSpeechProgress {
  loaded: number
  total: number
  /** The part of `loaded` that came over the network. */
  fetched: number
}

type OutMsg =
  | ({ type: 'progress' } & LocalSpeechProgress & { models?: string[] })
  | { type: 'ready' }
  | { type: 'fatal'; message: string }
  | { type: 'res'; id: number; status: number; headers: Record<string, string> }
  | { type: 'chunk'; id: number; data: ArrayBuffer }
  | { type: 'end'; id: number }

interface Pending {
  onPcm: (pcm: Float32Array) => void
  resolve: () => void
  reject: (error: unknown) => void
  status?: number
  error: Uint8Array[]
}

/** Thrown when the engine cannot load or answer: its own words, for the user. */
export class LocalSpeechEngineError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'LocalSpeechEngineError'
  }
}

const abortError = () => new DOMException('The operation was aborted.', 'AbortError')

export class LocalSpeechEngine {
  /** Settles once the models are on the GPU and their shaders compiled. */
  readonly ready: Promise<void>
  private readonly worker: Worker
  private readonly codec: Worker
  private readonly pending = new Map<number, Pending>()
  private nextId = 1
  private loaded = false
  private closed = false
  private settle!: { resolve: () => void; reject: (error: Error) => void }

  constructor(
    onProgress: (progress: LocalSpeechProgress) => void,
    /** A failure after loading, such as a lost GPU device. The engine is closed by then. */
    private readonly onFatal: (message: string) => void,
    files = localSpeechBase,
  ) {
    this.ready = new Promise<void>((resolve, reject) => (this.settle = { resolve, reject }))
    this.worker = new Worker(`${files}worker.js`, { type: 'module', name: 'ircord-voice-model' })
    this.codec = new Worker(`${files}codec-worker.js`, {
      type: 'module',
      name: 'ircord-voice-codec',
    })
    this.worker.onmessage = (event: MessageEvent<OutMsg>) => this.receive(event.data, onProgress)
    this.worker.onerror = (event) => this.fail(event.message || 'the engine worker did not start')
    this.codec.onerror = (event) =>
      this.fail(`the codec worker failed: ${event.message || 'did not start'}`)
    // The codec worker runs the audio decoder's last steps; the two talk over their own channel.
    const channel = new MessageChannel()
    this.codec.postMessage({ type: 'port' }, [channel.port2])
    const base = new URL(localSpeechModelsPath, location.href).href
    // No voice clips: the shipped embeddings and codes are all read-aloud needs.
    this.worker.postMessage(
      { type: 'init', base, cache: true, voicesPcm: {}, codecPort: channel.port1 },
      [channel.port1],
    )
  }

  private receive(m: OutMsg, onProgress: (progress: LocalSpeechProgress) => void) {
    switch (m.type) {
      case 'progress':
        if (!this.loaded) onProgress({ loaded: m.loaded, total: m.total, fetched: m.fetched })
        break
      case 'ready':
        this.loaded = true
        this.settle.resolve()
        break
      case 'fatal':
        this.fail(m.message)
        break
      case 'res': {
        const p = this.pending.get(m.id)
        if (p) p.status = m.status
        break
      }
      case 'chunk': {
        const p = this.pending.get(m.id)
        if (!p) break
        if (p.status === 200) p.onPcm(new Float32Array(m.data))
        else p.error.push(new Uint8Array(m.data))
        break
      }
      case 'end': {
        const p = this.pending.get(m.id)
        if (!p) break
        this.pending.delete(m.id)
        if (p.status === 200) p.resolve()
        else p.reject(new LocalSpeechEngineError(errorDetail(p)))
        break
      }
    }
  }

  private fail(message: string) {
    if (this.closed) return
    // Before loading, this rejects `ready`; after, the owner hears about it.
    this.terminate(new LocalSpeechEngineError(message))
    if (this.loaded) this.onFatal(message)
  }

  /** Generates one line, handing each float32 PCM chunk to `onPcm` as it arrives. */
  generate(body: object, signal: AbortSignal, onPcm: (pcm: Float32Array) => void) {
    return new Promise<void>((resolve, reject) => {
      if (this.closed) return reject(new LocalSpeechEngineError('the voice model was unloaded'))
      if (signal.aborted) return reject(abortError())
      const id = this.nextId++
      const done = () => signal.removeEventListener('abort', aborted)
      const aborted = () => {
        if (!this.pending.delete(id)) return
        this.worker.postMessage({ type: 'abort', id })
        reject(abortError())
      }
      signal.addEventListener('abort', aborted, { once: true })
      this.pending.set(id, {
        onPcm,
        error: [],
        resolve: () => (done(), resolve()),
        reject: (error) => (done(), reject(error)),
      })
      const bytes = new TextEncoder().encode(JSON.stringify(body)).buffer
      this.worker.postMessage(
        {
          type: 'req',
          id,
          method: 'POST',
          path: '/v2/generate',
          headers: { 'content-type': 'application/json' },
          body: bytes,
        },
        [bytes],
      )
    })
  }

  /** Stops both workers, which frees the model's graphics and system memory. */
  terminate(reason: Error = new LocalSpeechEngineError('the voice model was unloaded')) {
    if (this.closed) return
    this.closed = true
    this.worker.terminate()
    this.codec.terminate()
    for (const p of this.pending.values()) p.reject(reason)
    this.pending.clear()
    if (!this.loaded) this.settle.reject(reason)
  }
}

/** The engine's error answer, `{"detail": "..."}`, as text. */
function errorDetail(p: Pending) {
  const text = new TextDecoder().decode(
    p.error.reduce((all, part) => {
      const next = new Uint8Array(all.length + part.length)
      next.set(all)
      next.set(part, all.length)
      return next
    }, new Uint8Array()),
  )
  try {
    const detail = (JSON.parse(text) as { detail?: unknown }).detail
    if (typeof detail === 'string') return detail
    if (detail) return JSON.stringify(detail)
  } catch {
    /* Not JSON: use the text. */
  }
  return text.trim().slice(0, 300) || `the voice model answered ${p.status}`
}
