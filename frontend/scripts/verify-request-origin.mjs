// Run against an isolated demo or live-mode dev server on BASE. For live mode, point
// NUXT_BACKEND_URL at the disposable backend started here (http://127.0.0.1:4175).
import assert from 'node:assert/strict'
import { createServer, request as httpRequest } from 'node:http'
import { chromium } from 'playwright'

const base = process.env.BASE ?? 'http://127.0.0.1:4174'
const live = process.env.MODE === 'live'
const origin = process.env.PUBLIC_ORIGIN ?? base
let calls = 0
let writes = 0
let library = null
const backend =
  live &&
  createServer(async (req, res) => {
    calls++
    const path = new URL(req.url, 'http://localhost').pathname
    if (path === '/api/v1/auth/session')
      return res
        .writeHead(200, { 'content-type': 'application/json' })
        .end(JSON.stringify({ account: { id: 'fixture' } }))
    if (path === '/api/v1/app-data/library') {
      if (req.method === 'PUT') {
        writes++
        let body = ''
        for await (const chunk of req) body += chunk
        library = JSON.parse(body).data
      }
      return res
        .writeHead(200, { 'content-type': 'application/json' })
        .end(JSON.stringify({ data: library, revision: writes }))
    }
    if (req.method !== 'GET') writes++
    res.writeHead(200, { 'content-type': 'text/plain' }).end('fixture')
  })

if (backend) await new Promise((resolve) => backend.listen(4175, '127.0.0.1', resolve))
try {
  let cookie = 'ircord_session=disposable-fixture'
  if (!live) {
    const signup = await fetch(`${base}/api/v1/auth/signup`, {
      method: 'POST',
      headers: { origin, 'content-type': 'application/json' },
      body: JSON.stringify({ username: `Fixture${Date.now()}`, password: 'disposable-password' }),
    })
    assert.equal(signup.status, 201)
    cookie = signup.headers.get('set-cookie')?.split(';')[0]
    assert.ok(cookie)
  }
  const gif = {
    provider: 'klipy',
    id: 'origin-fixture',
    slug: 'fixture',
    title: 'fixture',
    url: 'https://static.klipy.com/a',
    previewUrl: 'https://static.klipy.com/a',
    stillUrl: 'https://static.klipy.com/a',
    width: 1,
    height: 1,
  }
  const request = (path, method, headers = {}, body) =>
    fetch(`${base}${path}`, {
      method,
      headers: { cookie, ...headers },
      body,
    })
  const gifBody = () => new Blob([JSON.stringify(gif)])
  const blocked = [
    { origin: 'https://sibling.example', 'sec-fetch-site': 'same-site' },
    { origin: 'https://foreign.example', 'sec-fetch-site': 'cross-site' },
    { origin: 'null' },
    { origin: 'https://bad.example/path' },
    { origin: '' },
    { origin: `${origin}/` },
    { 'sec-fetch-site': 'cross-site' },
    { 'sec-fetch-site': 'same-origin' },
    { 'sec-fetch-site': 'none' },
    {
      origin: 'https://foreign.example',
      'x-forwarded-host': 'foreign.example',
      'x-forwarded-proto': 'https',
      ...(live ? { host: 'foreign.example' } : {}),
    },
  ]
  for (const headers of blocked) {
    const before = calls
    const res = await request('/api/app/library/gifs', 'POST', headers, gifBody())
    assert.equal(res.status, 403)
    assert.equal(calls, before, 'blocked write reached backend')
  }
  for (const [path, method] of [
    ['/api/app/appearance', 'PUT'],
    ['/api/app/preferences', 'PUT'],
    ['/api/app/library', 'PATCH'],
    ['/api/app/library/gifs/example', 'DELETE'],
    ['/api/v1/push/subscriptions', 'POST'],
    ['/api/v1/auth/login', 'POST'],
  ]) {
    const before = calls
    assert.equal((await request(path, method, { origin: 'https://foreign.example' })).status, 403)
    assert.equal(calls, before, 'blocked request reached backend')
  }
  const beforeWrite = await request('/api/app/library', 'GET')
  assert.equal(beforeWrite.status, 200)
  assert.ok(
    !(await beforeWrite.json()).gifs.some((item) => item.id === gif.id),
    'blocked request changed storage',
  )
  if (!live) {
    // A different port is cross-origin but same-site, so SameSite=Lax sends the cookie.
    const attacker = createServer((_req, res) => res.end('<!doctype html><title>fixture</title>'))
    let browserCookieSent = false
    const proxy = createServer((req, res) => {
      browserCookieSent = !!req.headers.cookie
      const upstream = httpRequest(
        `${base}${req.url}`,
        {
          method: req.method,
          headers: req.headers,
        },
        (reply) => {
          res.writeHead(reply.statusCode, reply.headers)
          reply.pipe(res)
        },
      )
      req.pipe(upstream)
    })
    await new Promise((resolve) => attacker.listen(4176, '127.0.0.1', resolve))
    await new Promise((resolve) => proxy.listen(4177, '127.0.0.1', resolve))
    let browser
    try {
      browser = await chromium.launch({ headless: true })
      const context = await browser.newContext()
      const [name, value] = cookie.split('=')
      await context.addCookies([{ name, value, url: base, sameSite: 'Lax' }])
      const page = await context.newPage()
      await page.goto('http://127.0.0.1:4176/')
      const browserTarget = 'http://127.0.0.1:4177'
      const observed = page.waitForResponse(
        (response) => response.url() === `${browserTarget}/api/app/library/gifs`,
      )
      await page.evaluate(
        async ({ base, gif }) => {
          await fetch(`${base}/api/app/library/gifs`, {
            method: 'POST',
            mode: 'no-cors',
            credentials: 'include',
            body: new Blob([JSON.stringify({ ...gif, id: 'browser-fixture' })]),
          })
        },
        { base: browserTarget, gif },
      )
      const response = await observed
      assert.equal(response.status(), 403)
      assert.ok(browserCookieSent, 'browser did not send its cookie')
      const after = await request('/api/app/library', 'GET')
      assert.ok(!(await after.json()).gifs.some((item) => item.id === 'browser-fixture'))
    } finally {
      await browser?.close()
      await new Promise((resolve) => attacker.close(resolve))
      await new Promise((resolve) => proxy.close(resolve))
    }
  }
  assert.equal(
    (
      await request(
        '/api/app/library/gifs',
        'POST',
        { origin, 'sec-fetch-site': 'same-origin' },
        gifBody(),
      )
    ).status,
    200,
  )
  const saved = await request('/api/app/library', 'GET', {
    origin: 'null',
    'sec-fetch-site': 'cross-site',
  })
  assert.equal(saved.status, 200)
  assert.ok((await saved.json()).gifs.some((item) => item.id === gif.id))
  if (live) {
    assert.equal(writes, 1)
    assert.equal(
      (await request('/api/v1/push/test', 'POST', { origin, 'sec-fetch-site': 'same-origin' }))
        .status,
      200,
    )
    assert.equal(writes, 2, 'same-origin proxy write did not reach backend')
    assert.equal(
      (
        await request('/api/v1/link-media/widgets/fixture', 'GET', {
          origin: 'null',
          'sec-fetch-site': 'cross-site',
        })
      ).status,
      200,
    )
    assert.equal(
      (await request('/files/fixture', 'GET', { 'sec-fetch-site': 'cross-site' })).status,
      200,
    )
  }
  assert.equal((await request('/api/app/library/gifs', 'POST', {}, gifBody())).status, 200)
  console.log(`${live ? 'Live proxy' : 'Demo'} request-origin integration passed`)
} finally {
  if (backend) await new Promise((resolve) => backend.close(resolve))
}
