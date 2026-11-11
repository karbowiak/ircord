// Disposable demo, mocked durable replay, and Chromium lifecycle emulation.
// This does not measure an installed PWA on a physical Android device.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { chromium } from 'playwright'

const base = 'http://127.0.0.1:4180'
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
const frontend = spawn(
  process.execPath,
  [
    '-e',
    `
  import { runCommand } from '@nuxt/cli'
  await runCommand('dev', ['--host', '127.0.0.1', '--port', '4180', '--strictPort', '--no-fork', '--no-tui', '--no-takeover'], {
    overrides: {
      buildDir: '.data/nuxt-pwa-reconnect',
      typesDir: '.data/nuxt-pwa-reconnect',
      nitro: { devStorage: { data: { driver: 'memory' } } },
    },
  })
`,
  ],
  {
    env: { ...process.env, NUXT_DEMO: 'true', NUXT_BACKEND_URL: 'http://127.0.0.1:1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  },
)
let serverLog = ''
for (const stream of [frontend.stdout, frontend.stderr])
  stream.on('data', (chunk) => {
    serverLog = (serverLog + chunk).slice(-4000)
  })
let browser,
  context,
  created = false
let phase = 'startup'
let releaseHistory = () => {}
const errors = [],
  sockets = [],
  samples = []
try {
  for (let i = 0; ; i++) {
    assert(frontend.exitCode === null && i < 600, 'Isolated demo did not start')
    try {
      if ((await fetch(base)).ok) break
    } catch {
      /* startup */
    }
    await delay(100)
  }
  browser = await chromium.launch({ headless: true })
  context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
    serviceWorkers: 'block',
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `pwa${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  created = true
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  assert.equal(bootstrap.demo, true, 'This fixture must never reach the real backend')
  await context.addInitScript(() => {
    // Prevent automatic read mutations and keep lifecycle observations test-local.
    Object.defineProperty(document, 'hasFocus', { value: () => false })
    window.__pwaLifecycle = []
    for (const type of ['freeze', 'resume', 'visibilitychange'])
      document.addEventListener(type, () =>
        window.__pwaLifecycle.push({ type, at: performance.now() }),
      )
  })
  let holdBarrier = false
  await context.routeWebSocket(/\/api\/v1\/events\?/, (route) => {
    const record = { route, openedAt: performance.now(), closed: false }
    sockets.push(record)
    route.onClose(() => {
      record.closed = true
    })
    route.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      assert.equal(event.type, 'session.resume', 'Fixture accepts only replay commands')
      assert.equal(event.after, bootstrap.cursor, 'Resume keeps the applied durable cursor')
      record.resumedAt = performance.now()
      record.complete = () =>
        route.send(JSON.stringify({ v: 1, type: 'session.synced', cursor: event.after }))
      if (!holdBarrier) record.complete()
    })
    route.send(
      JSON.stringify({ v: 1, type: 'session.ready', sync: true, cursor: bootstrap.cursor }),
    )
  })
  const page = await context.newPage()
  page.setDefaultTimeout(10_000)
  page.on('pageerror', (error) => errors.push(error.message))
  const reads = []
  page.on('requestfinished', (request) => {
    const path = new URL(request.url()).pathname
    if (path === '/api/v1/auth/session' || path === '/api/v1/bootstrap')
      reads.push({
        phase: path.endsWith('bootstrap') ? 'bootstrap' : 'session',
        durationMs: request.timing().responseEnd,
      })
  })
  const chatState = () =>
    page.evaluate(() => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      return { status: chat.status, historyRefreshing: chat.historyRefreshing, cursor: chat.cursor }
    })
  const connected = () =>
    page.waitForFunction(
      () =>
        document
          .querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')?.status === 'connected',
    )
  const coldStart = performance.now()
  phase = 'cold connect'
  await page.goto(base)
  await connected()
  await page.locator('.message-row').first().waitFor()
  samples.push({
    scenario: 'cold demo load',
    connectedMs: performance.now() - coldStart,
    reads: [...reads],
  })
  assert.equal(sockets.length, 1)
  const initialBootstrapReads = reads.filter((r) => r.phase === 'bootstrap').length

  // Exercise the real store listeners with synthetic lifecycle events. A CDP probe
  // on this headless browser stayed visible and emitted no freeze/resume events;
  // suspended timer ordering is covered by verify-realtime-recovery.mjs instead.
  phase = 'synthetic freeze/resume'
  const historyRelease = new Promise((resolve) => {
    releaseHistory = resolve
  })
  let heldHistory = false
  await page.route('**/conversations/general/messages?*', async (route) => {
    const response = await route.fetch()
    heldHistory = true
    await historyRelease
    await route.fulfill({ response })
  })
  holdBarrier = true
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }))
    document.dispatchEvent(new Event('freeze'))
  })
  await delay(250)
  const thawStart = performance.now()
  await page.evaluate(() => {
    document.dispatchEvent(new Event('resume'))
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))
  })
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .status === 'syncing',
  )
  const readyMs = performance.now() - thawStart
  assert.equal(sockets.length, 2)
  assert(sockets[0].closed, 'Freeze retires the old generation')
  assert.equal((await chatState()).status, 'syncing', 'Ready alone does not complete recovery')
  await page.evaluate(() => {
    for (let i = 0; i < 5; i++) {
      window.dispatchEvent(new Event('focus'))
      window.dispatchEvent(new Event('online'))
      document.dispatchEvent(new Event('visibilitychange'))
      window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))
    }
  })
  assert.equal(sockets.length, 2, 'Lifecycle event bursts preserve one replacement')
  sockets.at(-1).complete()
  phase = 'replay barrier and history'
  await connected()
  const syncedMs = performance.now() - thawStart
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .historyRefreshing,
  )
  for (let i = 0; !heldHistory && i < 100; i++) await delay(10)
  assert(heldHistory, 'Recovery refreshes active history after the barrier')
  assert.equal(
    (await chatState()).status,
    'connected',
    'Held history does not gate the connected status',
  )
  await delay(250)
  releaseHistory()
  await page.waitForFunction(
    () =>
      !document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .historyRefreshing,
  )
  const lifecycle = await page.evaluate(() => window.__pwaLifecycle)
  assert(lifecycle.some((e) => e.type === 'freeze') && lifecycle.some((e) => e.type === 'resume'))
  samples.push({
    scenario: 'synthetic document freeze/resume and persisted pageshow',
    readyMs,
    syncedMs,
    historyCompleteMs: performance.now() - thawStart,
    lifecycle,
  })
  await page.unroute('**/conversations/general/messages?*')
  holdBarrier = false

  // Persisted pageshow must clear pagehide even when visibilitychange is absent.
  await page.evaluate(() =>
    window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true })),
  )
  phase = 'persisted pageshow'
  const showStart = performance.now()
  await page.evaluate(() =>
    window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true })),
  )
  await connected()
  assert.equal(sockets.length, 3)
  samples.push({
    scenario: 'persisted pageshow without visibilitychange',
    connectedMs: performance.now() - showStart,
  })
  assert.equal(
    reads.filter((r) => r.phase === 'bootstrap').length,
    initialBootstrapReads,
    'Warm lifecycle recovery does not bootstrap again',
  )
  assert.equal(sockets.filter((s) => !s.closed).length, 1)
  assert.deepEqual(errors, [])
  console.log(
    JSON.stringify({
      environment:
        'Headless desktop Chromium, mobile viewport, demo HTTP, mocked replay; no physical Android device',
      samples,
    }),
  )
} catch (error) {
  console.error(`Failed phase: ${phase}; mocked sockets: ${sockets.length}`)
  if (serverLog) console.error(serverLog)
  throw error
} finally {
  releaseHistory()
  if (created) await context?.request.delete(`${base}/api/v1/auth/account`).catch(() => {})
  await browser?.close()
  frontend.kill('SIGTERM')
}
