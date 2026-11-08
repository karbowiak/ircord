// Invoked by the Go integration fixture; uses isolated servers and native IRC peers.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { chromium } from 'playwright'
import validateEvent from '../shared/generated/event-validator.js'

const fixture = JSON.parse(process.env.IRCORD_READ_TEST)
const { api, ui, uiPort, username, password, accountId, networkId, room, nick, sender, control, token } = fixture
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
async function until(check, label) {
  for (let i = 0; i < 200; i++) {
    if (await check()) return
    await delay(50)
  }
  throw new Error(`Timed out: ${label}`)
}
async function native(path, data) {
  const response = await fetch(control + path, {
    method: data ? 'POST' : 'GET',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: data ? JSON.stringify(data) : undefined,
  })
  assert(response.ok, `native control ${path}: ${response.status}`)
  return response.status === 204 ? null : response.json()
}
async function send(text, target) {
  await delay(12) // Distinct server timestamps for partial-read assertions.
  await native('/send', { text, target })
  await until(async () => (await native('/state')).messages[text], `native received ${text}`)
}
async function read(text, target) {
  await native('/read', { text, target })
  await marker(text, target)
}
async function marker(text, target) {
  await until(async () => {
    const state = await native('/state')
    return state.markers[target === 'direct' ? sender : room] === state.messages[text]
  }, `native marker for ${text}`)
}
async function state(page, target = room) {
  return page.evaluate((target) => {
    const chat = document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
    return chat?.data?.conversations.find((c) => (c.target || c.name) === target)
  }, target)
}
async function counts(pages, unread, mentions, target = room) {
  for (const page of pages) {
    await until(async () => {
      const c = await state(page, target)
      return c?.unreadCount === unread && c?.mentionCount === mentions
    }, `browser ${target} unread=${unread} mentions=${mentions}`)
  }
}
async function serverView(page) {
  await page.getByRole('button', { name: 'Server', exact: true }).click()
}
async function markAll(page) {
  await page.getByRole('button', { name: 'Mark all read', exact: true }).click()
}
let uiLog = ''
const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
  env: {
    ...process.env, HOST: '127.0.0.1', PORT: String(uiPort), NUXT_DEMO: 'false',
    NUXT_BACKEND_URL: api, NUXT_PUBLIC_API_BASE: '/api/v1', NUXT_PUBLIC_NETWORK_PRESETS: '[]',
  }, stdio: ['ignore', 'pipe', 'pipe'],
})
frontend.stdout.on('data', (v) => { uiLog += v })
frontend.stderr.on('data', (v) => { uiLog += v })
let browser
const errors = []
let recovery
try {
  await until(async () => { try { return (await fetch(ui)).ok } catch { return false } }, 'isolated frontend')
  browser = await chromium.launch({ headless: true })
  const contexts = await Promise.all([browser.newContext({ serviceWorkers: 'block' }), browser.newContext({ serviceWorkers: 'block' })])
  const pages = []
  for (const context of contexts) {
    const response = await context.request.post(`${ui}/api/v1/auth/login`, { data: { username, password } })
    assert.equal(response.status(), 200)
    await context.addInitScript(({ accountId, networkId }) => {
      sessionStorage.setItem(`ircord.${accountId}.active`, JSON.stringify(`server:${networkId}`))
      // Deterministic focus conditions across two headless devices. The real
      // window focus listener is exercised below when the flag changes.
      window.__readTestFocused = false
      Object.defineProperty(document, 'hasFocus', { value: () => window.__readTestFocused })
    }, { accountId, networkId })
    const page = await context.newPage()
    page.on('pageerror', (e) => errors.push(e.message))
    page.on('websocket', (socket) => socket.on('framereceived', (frame) => {
      try {
        const event = JSON.parse(String(frame.payload))
        if (!validateEvent(event)) errors.push(`Invalid live event: ${event.type}`)
      } catch { errors.push('Invalid live JSON') }
    }))
    await page.goto(ui)
    await page.waitForFunction(() => document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')?.status === 'connected')
    pages.push(page)
  }
  const [a, b] = pages
  const conversation = await state(a)
  assert(conversation?.joined)
  const path = `/api/v1/conversations/${conversation.id}`
  const info = await (await contexts[0].request.get(`${ui}/api/v1/networks/${networkId}/connection`)).json()
  for (const cap of ['draft/read-marker', 'server-time', 'draft/chathistory', 'sasl']) assert(info.negotiatedCapabilities.includes(cap), `negotiated ${cap}`)
  console.log('Two authenticated IRCord browsers and a native client share one upstream SASL account.')

  const first = `${nick}: first mention`, middle = 'middle message', last = `${nick}: last mention`
  for (const text of [first, middle, last]) await send(text)
  await counts(pages, 3, 2)
  await read(first)
  await counts(pages, 2, 1)
  assert.equal(await a.locator('.conversation-link').filter({ hasText: room.slice(1) }).locator('.mention-badge').innerText(), '1')
  await native('/read', { text: first })
  await counts(pages, 2, 1)
  await read(last)
  await counts(pages, 0, 0)
  await send('read from IRCord')
  await counts(pages, 1, 0)
  await markAll(a)
  await counts(pages, 0, 0)
  await marker('read from IRCord')
  console.log('Partial/native reads, duplicate markers, mentions and IRCord mark-all-read synchronize.')

  await a.locator('.conversation-link').filter({ hasText: room.slice(1) }).click()
  assert.equal(await a.evaluate(() => document.visibilityState), 'visible')
  await send('unfocused visible conversation')
  await counts(pages, 1, 0)
  await delay(700)
  await counts(pages, 1, 0)
  await a.evaluate(() => { window.__readTestFocused = true; window.dispatchEvent(new Event('focus')) })
  await counts(pages, 0, 0)
  await marker('unfocused visible conversation')
  await serverView(a)
  console.log('A visible unfocused window preserves unread state; focus clears it on both devices.')

  await send('read before delayed HTTP response')
  await counts(pages, 1, 0)
  let releaseResponse, responseHeld
  const held = new Promise((resolve) => { responseHeld = resolve })
  const release = new Promise((resolve) => { releaseResponse = resolve })
  await a.route(`**${path}`, async (route) => {
    if (route.request().method() !== 'PATCH') return route.continue()
    const response = await route.fetch()
    responseHeld()
    await release
    await route.fulfill({ response })
  }, { times: 1 })
  await markAll(a)
  await held
  await marker('read before delayed HTTP response')
  await send(`${nick}: arrived after mark-read committed`)
  await counts(pages, 1, 1)
  releaseResponse()
  await delay(400)
  await counts(pages, 1, 1)
  await a.route(`**${path}`, (route) => route.fulfill({ status: 503, contentType: 'application/json', body: '{}' }), { times: 1 })
  await markAll(a)
  await a.getByText('Couldn’t mark this conversation read. Try again.', { exact: true }).waitFor()
  await counts(pages, 1, 1)
  await markAll(a)
  await counts(pages, 0, 0)
  console.log('Delayed HTTP responses preserve newer live unread counts; failed reads roll back and can retry.')

  await send('before reload')
  await counts(pages, 1, 0)
  await b.reload()
  await counts(pages, 1, 0)
  await contexts[1].setOffline(true)
  await read('before reload')
  await counts([a], 0, 0)
  await send(`${nick}: during offline websocket`)
  await send('another during offline websocket')
  await counts([a], 2, 1)
  await contexts[1].setOffline(false)
  await counts(pages, 2, 1)
  await markAll(a)
  await counts(pages, 0, 0)
  console.log('Reload and offline WebSocket replay restore canonical unread and mention counts.')

  await send('read locally while upstream disconnected')
  await counts(pages, 1, 0)
  assert.equal((await contexts[0].request.patch(`${ui}/api/v1/networks/${networkId}`, { data: { status: 'disconnected' } })).status(), 200)
  await markAll(a)
  await counts(pages, 0, 0)
  assert.equal((await contexts[0].request.patch(`${ui}/api/v1/networks/${networkId}`, { data: { status: 'connected' } })).status(), 200)
  await marker('read locally while upstream disconnected')
  console.log('A local read made while IRC was disconnected reconciles to the native client after reconnect.')

  assert.equal((await contexts[0].request.patch(`${ui}/api/v1/networks/${networkId}`, { data: { status: 'disconnected' } })).status(), 200)
  await send(`${nick}: channel while upstream disconnected`)
  await read(`${nick}: channel while upstream disconnected`)
  await send('direct while IRCord disconnected', 'direct')
  await read('direct while IRCord disconnected', 'direct')
  assert.equal((await contexts[0].request.patch(`${ui}/api/v1/networks/${networkId}`, { data: { status: 'connected' } })).status(), 200)
  await until(async () => (await state(a))?.joined, 'upstream rejoin')
  for (const page of pages) {
    await until(async () => {
      const snapshot = await (await page.request.get(`${ui}/api/v1/bootstrap`)).json()
      const direct = snapshot.conversations.find((c) => c.kind === 'direct' && c.name === sender)
      recovery = { networks: snapshot.networks.map((n) => ({ status: n.status })), conversations: snapshot.conversations }
      if (!direct || snapshot.networks[0]?.status !== 'connected') return false
      const history = await (await page.request.get(`${ui}/api/v1/conversations/${conversation.id}/messages?limit=100`)).json()
      const directs = await (await page.request.get(`${ui}/api/v1/conversations/${direct.id}/messages?limit=100`)).json()
      recovery.history = history.items?.map((m) => m.text)
      recovery.directs = directs.items?.map((m) => m.text)
      return directs.items.some((m) => m.text === 'direct while IRCord disconnected') && history.items.some((m) => m.text === `${nick}: channel while upstream disconnected`)
    }, 'channel and DM history recovered')
  }
  await counts(pages, 0, 0)
  await counts(pages, 0, 0, sender)
  await send('new direct after recovery', 'direct')
  await counts(pages, 1, 1, sender)
  await markAll(b)
  await counts(pages, 0, 0, sender)
  await marker('new direct after recovery', 'direct')
  console.log('Upstream reconnect imports channel/DM history without resurrecting messages read by the native client.')
  assert.deepEqual(errors, [])
  console.log('PASS: read-marker devices/native integration; all observed WebSocket frames match the Go contract.')
} catch (error) {
  console.error(uiLog.slice(-2000))
  if (recovery) console.error(JSON.stringify(recovery))
  throw error
} finally {
  await browser?.close()
  frontend.kill('SIGTERM')
  await Promise.race([once(frontend, 'exit'), delay(3000)])
}
