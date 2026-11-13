// Exercise the real routes through the installed development and production adapters.
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { createRequire } from 'node:module'
import { serve } from 'bun'
import WebSocket from 'ws'
import currentAdapter from 'crossws/adapters/bun'

const nitroRequire = createRequire(
  new URL('../node_modules/nitropack/package.json', import.meta.url),
)
const { default: legacyAdapter } = await import(nitroRequire.resolve('crossws/adapters/node'))
let demo = false
let upstreamURL = ''
globalThis.defineWebSocketHandler = (hooks) => hooks
globalThis.useRuntimeConfig = () => ({ demo, backendUrl: upstreamURL, siteUrl: '' })
globalThis.accountForToken = async (token) => (token ? { id: token } : null)

const events = (await import('../server/routes/api/v1/events.ts')).default
const trace = (await import('../server/routes/api/v1/networks/[id]/connection/trace/events.ts'))
  .default
const upstream = serve({
  port: 0,
  fetch(request, server) {
    if (request.headers.get('cookie') !== 'ircord_session=valid')
      return new Response('Unauthorized', { status: 401 })
    if (server.upgrade(request)) return
    return new Response('Upgrade failed', { status: 500 })
  },
  websocket: {
    open(socket) {
      socket.send('early')
    },
    message(socket, message) {
      socket.send(message)
    },
  },
})
upstreamURL = `http://127.0.0.1:${upstream.port}`
const waitFor = async (test) => {
  for (let i = 0; i < 200; i++) {
    if (test()) return
    await new Promise((resolve) => setTimeout(resolve, 10))
  }
  throw new Error('Timed out waiting for a fixture socket')
}

try {
  for (const [createAdapter, isBun] of [
    [legacyAdapter, false],
    [currentAdapter, true],
  ]) {
    const adapter = createAdapter({
      resolve(info) {
        return (info.request?.url || info.url).includes('/trace/events') ? trace : events
      },
    })
    let server
    if (isBun) {
      server = serve({
        port: 0,
        fetch: (request, listener) => adapter.handleUpgrade(request, listener),
        websocket: adapter.websocket,
      })
    } else {
      server = createServer()
      server.on('upgrade', adapter.handleUpgrade)
      await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
    }
    const origin = `http://127.0.0.1:${isBun ? server.port : server.address().port}`
    const sockets = new Set()
    const connect = (path, { cookie = 'valid', socketOrigin = origin } = {}) =>
      new Promise((resolve, reject) => {
        const socket = new WebSocket(`${origin.replace('http:', 'ws:')}${path}`, {
          headers: { Origin: socketOrigin, Cookie: `ircord_session=${cookie}` },
          handshakeTimeout: 2000,
        })
        const messages = []
        sockets.add(socket)
        socket.on('message', (message) => messages.push(message.toString()))
        socket.on('open', () => resolve({ socket, messages, status: 101 }))
        socket.on('unexpected-response', (_request, response) => {
          response.resume()
          resolve({ socket, messages, status: response.statusCode })
        })
        socket.on('error', reject)
      })
    try {
      const first = await connect('/api/v1/events')
      const second = await connect('/api/v1/events')
      await waitFor(() => first.messages.includes('early') && second.messages.includes('early'))
      first.socket.send('first-only')
      second.socket.send('second-only')
      await waitFor(
        () => first.messages.includes('first-only') && second.messages.includes('second-only'),
      )
      assert(!first.messages.includes('second-only'))
      assert(!second.messages.includes('first-only'))

      const diagnostic = await connect('/api/v1/networks/test/connection/trace/events')
      await waitFor(() => diagnostic.messages.includes('early'))
      let closeCode
      diagnostic.socket.once('close', (code) => {
        closeCode = code
      })
      diagnostic.socket.send('not allowed')
      await waitFor(() => closeCode !== undefined)
      assert.equal(closeCode, 1008)
      for (const path of ['/api/v1/events', '/api/v1/networks/test/connection/trace/events']) {
        assert.equal((await connect(path, { cookie: 'bad' })).status, 401)
        assert.equal((await connect(path, { socketOrigin: 'https://invalid.example' })).status, 403)
      }

      demo = true
      const demoFirst = await connect('/api/v1/events', { cookie: 'demo-one' })
      const demoSecond = await connect('/api/v1/events', { cookie: 'demo-two' })
      await waitFor(() => demoFirst.messages.length && demoSecond.messages.length)
      assert.equal(JSON.parse(demoFirst.messages[0]).type, 'session.ready')
      assert.equal(JSON.parse(demoSecond.messages[0]).type, 'session.ready')
      demoFirst.socket.send(JSON.stringify({ v: 1, type: 'session.resume', after: -1 }))
      await waitFor(() => demoFirst.messages.length > 1)
      assert.equal(JSON.parse(demoFirst.messages[1]).type, 'command.error')
      assert.equal(demoSecond.messages.length, 1)
      demo = false
    } finally {
      demo = false
      for (const socket of sockets) socket.terminate()
      if (isBun) {
        await adapter.close(1001, 'test complete')
        server.stop(true)
      } else {
        adapter.closeAll(1001, 'test complete', true)
        await new Promise((resolve) => server.close(resolve))
      }
    }
  }
  console.log(
    'Legacy and current CrossWS adapters, demo and live isolation, bridge flow, and rejection pass.',
  )
} finally {
  upstream.stop(true)
}
