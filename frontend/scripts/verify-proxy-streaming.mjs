#!/usr/bin/env bun
// Run after `bun run build`. Starts the production server against a stub backend on free local
// ports and checks that the proxy streams: each side sees the first bytes before the other side
// has finished, and a signed-out write is answered without its body being read.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { createServer } from 'node:http'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const entry = resolve(frontend, '.output/server/index.mjs')

const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
/** Resolves when `signal` names it; the stub backend and the client wait on each other. */
const signals = new Map()
const signal = (name) => {
  if (!signals.has(name)) {
    let resolve
    const promise = new Promise((done) => (resolve = done))
    signals.set(name, { promise, resolve })
  }
  return signals.get(name)
}
const within = (promise, ms, what) => {
  let timer
  return Promise.race([
    promise,
    new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(`${what} timed out`)), ms)
    }),
  ]).finally(() => clearTimeout(timer))
}

let sessionCalls = 0
const backend = createServer(async (req, res) => {
  const url = new URL(req.url, 'http://localhost')
  if (url.pathname === '/files/held') {
    // Sends one chunk, then holds the rest until the client confirms it has that chunk.
    res.writeHead(200, { 'content-type': 'application/octet-stream' })
    res.write('first chunk;')
    await signal('download-first').promise
    return res.end('rest')
  }
  if (url.pathname === '/files/range') {
    return res
      .writeHead(206, {
        'content-type': 'video/mp4',
        'content-range': 'bytes 2-5/10',
        'accept-ranges': 'bytes',
        'content-length': '4',
      })
      .end('2345')
  }
  if (url.pathname === '/api/v1/upload') {
    let received = ''
    for await (const chunk of req) {
      received += chunk
      if (received.includes('first part;')) signal('upload-first').resolve()
    }
    return res.writeHead(200, { 'content-type': 'application/json' }).end(
      JSON.stringify({
        received,
        forwardedFor: req.headers['x-forwarded-for'] ?? '',
        forwardedHost: req.headers['x-forwarded-host'] ?? '',
      }),
    )
  }
  if (url.pathname === '/api/v1/auth/session') {
    sessionCalls++
    return res
      .writeHead(401, { 'content-type': 'application/json' })
      .end(JSON.stringify({ title: 'Unauthorized' }))
  }
  if (url.pathname === '/api/v1/empty') return res.writeHead(204).end()
  if (url.pathname === '/api/v1/cookies')
    return res
      .writeHead(200, {
        'content-type': 'application/json',
        'set-cookie': ['first=1; Path=/; HttpOnly', 'second=2; Path=/'],
      })
      .end('{}')
  if (url.pathname === '/api/v1/missing')
    return res.writeHead(404, { 'content-type': 'application/json' }).end('{"title":"Missing"}')
  res.writeHead(500).end()
})
await new Promise((done) => backend.listen(0, '127.0.0.1', done))

const freePort = await new Promise((done) => {
  const probe = createServer().listen(0, '127.0.0.1', () => {
    const { port } = probe.address()
    probe.close(() => done(port))
  })
})
const base = `http://127.0.0.1:${freePort}`
const server = spawn(process.execPath, [entry], {
  env: {
    ...process.env,
    HOST: '127.0.0.1',
    PORT: String(freePort),
    NUXT_DEMO: 'false',
    NUXT_BACKEND_URL: `http://127.0.0.1:${backend.address().port}`,
    NUXT_SITE_URL: base,
  },
  stdio: ['ignore', 'ignore', 'pipe'],
})
// The signed-out requests below log errors; show the server's output only when a check fails.
let serverLog = ''
server.stderr.on('data', (chunk) => (serverLog = (serverLog + chunk).slice(-4000)))

try {
  for (let attempt = 0; ; attempt++) {
    try {
      await fetch(`${base}/robots.txt`)
      break
    } catch (error) {
      if (attempt > 100) throw error
      await delay(100)
    }
  }

  // A download's first bytes reach the client while the backend is still sending.
  {
    const response = await within(fetch(`${base}/files/held`), 5000, 'Download headers')
    assert.equal(response.status, 200)
    assert.equal(response.headers.get('x-frame-options'), 'DENY', 'route rule headers are kept')
    const reader = response.body.getReader()
    const decoder = new TextDecoder()
    let text = ''
    while (!text.includes('first chunk;')) {
      const { done, value } = await within(reader.read(), 5000, 'First download chunk')
      assert.ok(!done, 'download ended before its first chunk')
      text += decoder.decode(value, { stream: true })
    }
    signal('download-first').resolve()
    for (;;) {
      const { done, value } = await within(reader.read(), 5000, 'Download end')
      if (done) break
      text += decoder.decode(value, { stream: true })
    }
    assert.equal(text, 'first chunk;rest')
  }

  // An upload's first bytes reach the backend while the client is still sending.
  {
    const encoder = new TextEncoder()
    let sent = 0
    const body = new ReadableStream({
      async pull(controller) {
        if (sent++ === 0) return controller.enqueue(encoder.encode('first part;'))
        await within(signal('upload-first').promise, 5000, 'Upload reaching the backend')
        controller.enqueue(encoder.encode('second part'))
        controller.close()
      },
    })
    const response = await within(
      fetch(`${base}/api/v1/upload`, {
        method: 'POST',
        headers: { origin: base, 'content-type': 'application/octet-stream' },
        body,
        duplex: 'half',
      }),
      10000,
      'Upload',
    )
    assert.equal(response.status, 200)
    const result = await response.json()
    assert.equal(result.received, 'first part;second part')
    assert.match(result.forwardedFor, /127\.0\.0\.1$/, 'the peer address reaches the backend')
    assert.equal(result.forwardedHost, `127.0.0.1:${freePort}`)
  }

  // A signed-out write is answered without waiting for its body.
  {
    let ended = false
    const body = new ReadableStream({
      async pull(controller) {
        controller.enqueue(new TextEncoder().encode('{"preferences":'))
        await signal('signed-out-answered').promise
        ended = true
        controller.close()
      },
    })
    const response = await within(
      fetch(`${base}/api/app/preferences`, {
        method: 'PUT',
        headers: { origin: base, 'content-type': 'application/json' },
        body,
        duplex: 'half',
      }),
      10000,
      'Signed-out write',
    )
    assert.equal(response.status, 401)
    assert.ok(!ended, 'the request body was read before the account check')
    assert.ok(sessionCalls > 0)
  }

  // The account check still happens before a body that has arrived is read.
  {
    const before = sessionCalls
    const response = await fetch(`${base}/api/app/appearance`, {
      method: 'PUT',
      headers: { origin: base, 'content-type': 'application/json' },
      body: JSON.stringify({ settings: {} }),
    })
    assert.equal(response.status, 401)
    assert.equal(sessionCalls, before + 1)
  }

  // Statuses, ranges, cookies and errors pass through unchanged.
  {
    const range = await fetch(`${base}/files/range`, { headers: { range: 'bytes=2-5' } })
    assert.equal(range.status, 206)
    assert.equal(range.headers.get('content-range'), 'bytes 2-5/10')
    assert.equal(range.headers.get('accept-ranges'), 'bytes')
    assert.equal(await range.text(), '2345')

    const empty = await fetch(`${base}/api/v1/empty`, {
      method: 'DELETE',
      headers: { origin: base },
    })
    assert.equal(empty.status, 204)
    assert.equal(await empty.text(), '')

    const head = await fetch(`${base}/files/range`, { method: 'HEAD' })
    assert.equal(await head.text(), '')

    const cookies = await fetch(`${base}/api/v1/cookies`)
    assert.deepEqual(cookies.headers.getSetCookie(), [
      'first=1; Path=/; HttpOnly',
      'second=2; Path=/',
    ])

    const missing = await fetch(`${base}/api/v1/missing`)
    assert.equal(missing.status, 404)
    assert.deepEqual(await missing.json(), { title: 'Missing' })

    const foreign = await fetch(`${base}/api/v1/upload`, {
      method: 'POST',
      headers: { origin: 'https://foreign.example' },
      body: 'x',
    })
    assert.equal(foreign.status, 403)
  }

  // Bodies over the server's cap are refused.
  {
    const response = await fetch(`${base}/api/v1/upload`, {
      method: 'POST',
      headers: { origin: base, 'content-type': 'application/octet-stream' },
      body: new Uint8Array((32 << 20) + 1),
    }).catch((error) => error)
    assert.ok(
      response instanceof Error || response.status === 413,
      `expected an oversized body to be refused, got ${response.status}`,
    )
  }

  console.log('The production proxy streams downloads and uploads and keeps responses unchanged.')
} catch (error) {
  console.error(serverLog)
  throw error
} finally {
  signal('signed-out-answered').resolve()
  server.kill()
  backend.close()
}
