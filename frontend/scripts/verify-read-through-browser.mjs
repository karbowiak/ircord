// Launched by TestReadThroughBrowser with an isolated backend and frontend build.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { chromium } from 'playwright'

const {
  api,
  ui,
  uiPort,
  owner,
  username,
  password,
  control,
  frontend: build,
} = JSON.parse(process.env.IRCORD_READ_THROUGH_TEST)
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
async function until(check, label) {
  for (let i = 0; i < 400; i++) {
    if (await check()) return
    await delay(25)
  }
  throw new Error(`Timed out: ${label}`)
}
function deferred() {
  let resolve
  const promise = new Promise((done) => {
    resolve = done
  })
  return { promise, resolve }
}
const frontend = spawn(process.execPath, [`${build}/.output/server/index.mjs`], {
  env: {
    ...process.env,
    HOST: '127.0.0.1',
    PORT: String(uiPort),
    NUXT_DEMO: 'false',
    NUXT_BACKEND_URL: api,
    NUXT_PUBLIC_API_BASE: '/api/v1',
    NUXT_PUBLIC_NETWORK_PRESETS: '[]',
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
let log = ''
frontend.stdout.on('data', (chunk) => {
  log = (log + chunk).slice(-3000)
})
frontend.stderr.on('data', (chunk) => {
  log = (log + chunk).slice(-3000)
})
let browser
try {
  await until(async () => {
    try {
      return (await fetch(ui)).ok
    } catch {
      return false
    }
  }, 'frontend startup')
  browser = await chromium.launch({ headless: true })
  async function open(room, options = {}) {
    const context = await browser.newContext({ serviceWorkers: 'block' })
    const login = await context.request.post(`${ui}/api/v1/auth/login`, {
      data: { username, password },
    })
    assert.equal(login.status(), 200)
    await context.addInitScript(
      ({ owner, room }) => {
        localStorage.removeItem(`ircord.snapshot.${owner}`)
        sessionStorage.setItem(`ircord.${owner}.active`, JSON.stringify(room))
        window.__readFocused = false
        Object.defineProperty(document, 'hasFocus', { value: () => window.__readFocused })
        const nativeFetch = window.fetch
        window.fetch = function (resource, init) {
          if (String(resource).includes('/openapi.json')) window.__discoverySignal = init?.signal
          return nativeFetch.call(this, resource, init)
        }
      },
      { owner, room },
    )
    const page = await context.newPage()
    const patches = []
    await page.route(`**/api/v1/conversations/${room}`, async (route) => {
      if (route.request().method() !== 'PATCH') return route.continue()
      const body = route.request().postDataJSON()
      patches.push(body)
      if (options.hold && body.readThroughMessageId) await options.hold(body).promise
      await route.continue().catch(() => {})
    })
    if (options.openapi) await page.route('**/openapi.json', options.openapi)
    await page.goto(`${ui}/channel/${room}`)
    await page.waitForFunction((room) => {
      const chat = document
        .querySelector('#__nuxt')
        ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
      return (
        chat?.status === 'connected' &&
        chat.activeId === room &&
        !!document.querySelector('.conversation-pane')?.textContent.includes('Observed A')
      )
    }, room)
    return { context, page, patches }
  }
  const focus = (page) =>
    page.evaluate(() => {
      window.__readFocused = true
      window.dispatchEvent(new Event('focus'))
    })
  const state = (page, room) =>
    page.evaluate(
      (room) =>
        document
          .querySelector('#__nuxt')
          .__vue_app__.config.globalProperties.$pinia._s.get('chat')
          .data.conversations.find((c) => c.id === room),
      room,
    )
  const holdA = deferred()
  const holdB = deferred()
  const live = await open('read-room', {
    hold: (body) => (body.readThroughMessageId === 'read-room-a' ? holdA : holdB),
  })
  await focus(live.page)
  await until(() => live.patches.length === 1, 'automatic read A')
  assert.deepEqual(live.patches[0], { readThroughMessageId: 'read-room-a' })
  assert.equal(
    (await state(live.page, 'read-room')).unreadCount,
    1,
    'automatic read must not optimistically clear counts',
  )
  assert.equal((await fetch(control, { method: 'POST' })).status, 204)
  await live.page.evaluate(() => {
    void document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .returnToPresent('read-room')
  })
  await live.page.getByText('Later B', { exact: true }).waitFor()
  holdA.resolve()
  await until(() => live.patches.length === 2, 'complete page schedules B')
  assert.deepEqual(live.patches[1], { readThroughMessageId: 'read-room-b' })
  const manual = await live.context.request.patch(`${ui}/api/v1/conversations/read-room`, {
    data: { unreadFrom: 'read-room-b' },
  })
  assert.equal(manual.status(), 200)
  await until(
    async () => (await state(live.page, 'read-room')).unreadFrom === 'read-room-b',
    'manual boundary event',
  )
  const bResponse = live.page.waitForResponse(
    (response) =>
      response.request().method() === 'PATCH' &&
      response.request().postDataJSON()?.readThroughMessageId === 'read-room-b',
  )
  holdB.resolve()
  assert.equal((await bResponse).status(), 200)
  const snapshot = await live.context.request.get(`${ui}/api/v1/bootstrap`)
  const canonical = (await snapshot.json()).conversations.find((c) => c.id === 'read-room')
  assert.equal(
    canonical.unreadFrom,
    'read-room-b',
    'server must preserve manual unread across the in-flight request',
  )
  assert.equal(canonical.unreadCount, 1)
  await until(
    async () => (await state(live.page, 'read-room')).unreadFrom === 'read-room-b',
    'manual boundary survives automatic read',
  )
  await live.page.evaluate(() => {
    void document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .markRead('read-room')
  })
  await until(() => live.patches.some((body) => body.read === true), 'explicit mark-all')
  assert.deepEqual(
    live.patches.find((body) => body.read === true),
    { read: true },
  )
  await until(
    async () => !(await state(live.page, 'read-room')).unreadFrom,
    'explicit read clears manual boundary',
  )
  await live.context.close()

  const old = await open('old-room', {
    openapi: (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }),
  })
  const discovery = old.page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/openapi.json',
  )
  await focus(old.page)
  await discovery
  await delay(500)
  assert.deepEqual(old.patches, [], 'unsupported server must not receive an automatic mark-all')
  await old.page.evaluate(() => {
    void document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .markRead('old-room')
  })
  await until(() => old.patches.length === 1, 'legacy explicit mark-all')
  assert.deepEqual(old.patches[0], { read: true })
  await old.context.close()

  const heldDiscovery = deferred()
  const discoveryStarted = deferred()
  const teardown = await open('dispose-room', {
    openapi: async (route) => {
      discoveryStarted.resolve()
      await heldDiscovery.promise
      await route
        .fulfill({ status: 200, contentType: 'application/json', body: '{}' })
        .catch(() => {})
    },
  })
  await focus(teardown.page)
  await discoveryStarted.promise
  assert.equal(await teardown.page.evaluate(() => window.__discoverySignal?.aborted), false)
  await teardown.page.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .dispose(),
  )
  assert.equal(
    await teardown.page.evaluate(() => window.__discoverySignal?.aborted),
    true,
    'store disposal aborts discovery',
  )
  heldDiscovery.resolve()
  await delay(350)
  assert.deepEqual(teardown.patches, [], 'disposed store must not send a delayed read')
  await teardown.context.close()
  console.log(
    'PASS: actual chat-store bounded reads, manual override, explicit read, legacy discovery, and teardown',
  )
} catch (error) {
  console.error(log)
  throw error
} finally {
  await browser?.close()
  frontend.kill('SIGTERM')
}
