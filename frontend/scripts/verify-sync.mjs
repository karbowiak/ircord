// Isolated Go fixture, production Nuxt build, and real Chromium. Run via the Go
// TestWorkspaceSyncBrowser fixture; measurements contain no account secrets.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { mkdir, writeFile } from 'node:fs/promises'
import { chromium } from 'playwright'
import WebSocket from 'ws'

const fixture = JSON.parse(process.env.IRCORD_SYNC_TEST)
const { api, ui, uiPort, owner, username, password, control, label = 'current' } = fixture
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
async function until(check, label) {
  for (let i = 0; i < 400; i++) {
    if (await check()) return
    await delay(25)
  }
  throw new Error(`Timed out: ${label}`)
}
const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
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
let serverLog = ''
frontend.stdout.on('data', (chunk) => {
  serverLog = (serverLog + chunk).slice(-2000)
})
frontend.stderr.on('data', (chunk) => {
  serverLog = (serverLog + chunk).slice(-2000)
})
let browser
const errors = []
const samples = []
try {
  await until(async () => {
    try {
      return (await fetch(ui)).ok
    } catch {
      return false
    }
  }, 'frontend startup')
  browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const login = await context.request.post(`${ui}/api/v1/auth/login`, {
    data: { username, password },
  })
  assert.equal(login.status(), 200)
  await context.addInitScript(
    ({ owner }) => {
      if (!new URL(location.href).searchParams.has('checkpoint'))
        localStorage.removeItem(`ircord.snapshot.${owner}`)
      sessionStorage.setItem(`ircord.${owner}.active`, JSON.stringify('sync-room-0'))
      Object.defineProperty(document, 'hasFocus', { value: () => false })
      window.__syncTiming = { workspace: null, messages: null, connected: null, seq: 0, frames: 0 }
      window.__syncSockets = []
      window.__syncEvents = []
      const NativeSocket = window.WebSocket
      window.WebSocket = class extends NativeSocket {
        constructor(...args) {
          super(...args)
          window.__syncSockets.push(this)
          this.addEventListener('message', ({ data }) => {
            const event = JSON.parse(data)
            window.__syncEvents.push(event)
            window.__syncTiming.seq = Math.max(window.__syncTiming.seq, event.seq || 0)
            window.__syncTiming.frames++
          })
        }
      }
      function observe() {
        const chat = document
          .querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
        const timings = window.__syncTiming
        if (
          !timings.workspace &&
          chat?.data?.conversations.length === 3 &&
          document.querySelectorAll('.conversation-link').length >= 3
        )
          timings.workspace = performance.now()
        if (
          !timings.messages &&
          document.querySelector('.conversation-pane')?.textContent.includes('Fixture message 199')
        )
          timings.messages = performance.now()
        if (!timings.connected && chat?.status === 'connected')
          timings.connected = performance.now()
        requestAnimationFrame(observe)
      }
      requestAnimationFrame(observe)
    },
    { owner },
  )
  const page = await context.newPage()
  const cookie = (await context.cookies(ui)).map(({ name, value }) => `${name}=${value}`).join('; ')
  const upgradeStatus = (path, headers) =>
    new Promise((resolve, reject) => {
      const socket = new WebSocket(`${ui.replace(/^http/, 'ws')}${path}`, {
        headers,
        handshakeTimeout: 5000,
      })
      socket.on('unexpected-response', (_request, response) => {
        response.resume()
        resolve(response.statusCode)
      })
      socket.on('open', () => {
        socket.close()
        resolve(101)
      })
      socket.on('error', reject)
    })
  for (const path of ['/api/v1/events', '/api/v1/networks/sync-network/connection/trace/events']) {
    assert.equal(
      await upgradeStatus(path, { Origin: 'https://invalid.example', Cookie: cookie }),
      403,
    )
    assert.equal(await upgradeStatus(path, { Origin: ui }), 401)
  }
  assert.equal(
    await upgradeStatus('/api/v1/networks/missing/connection/trace/events', {
      Origin: ui,
      Cookie: cookie,
    }),
    404,
  )
  let bootstrapRequests = 0
  page.on('request', (request) => {
    if (new URL(request.url()).pathname === '/api/v1/bootstrap') bootstrapRequests++
  })
  page.on('pageerror', (error) => errors.push(error.message))
  for (let run = 0; run < 11; run++) {
    await page.goto(ui)
    await page.waitForFunction(() => {
      const t = window.__syncTiming
      return t.workspace && t.messages && t.connected
    })
    if (run === 0) {
      const traceClose = await page.evaluate(
        () =>
          new Promise((resolve, reject) => {
            const trace = new WebSocket(
              `${location.origin.replace(/^http/, 'ws')}/api/v1/networks/sync-network/connection/trace/events`,
            )
            trace.onerror = () => reject(new Error('Trace socket failed to open'))
            trace.onopen = () => trace.send('invalid')
            trace.onclose = (event) => resolve(event.code)
          }),
      )
      assert.equal(traceClose, 1008, 'trace socket is receive-only')
    }
    const timings = await page.evaluate(() => ({ ...window.__syncTiming }))
    const historyStart = performance.now()
    await Promise.all(
      [0, 1, 2].map(async (i) => {
        const response = await context.request.get(
          `${ui}/api/v1/conversations/sync-room-${i}/messages?limit=40`,
        )
        assert.equal(response.status(), 200)
        const page = await response.json()
        assert.equal(page.items.length, 40)
      }),
    )
    const allHistoryMs = performance.now() - historyStart
    await page.evaluate(() => {
      for (const socket of window.__syncSockets) socket.close()
    })
    const mutation = await fetch(`${control}?run=${run}`, { method: 'POST' })
    assert.equal(mutation.status, 200)
    const { cursor } = await mutation.json()
    const resumeStart = await page.evaluate(() => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      const start = performance.now()
      chat.reconnect()
      return start
    })
    await page.waitForFunction(
      ({ cursor, run }) => {
        const chat = document
          .querySelector('#__nuxt')
          .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        return (
          chat.status === 'connected' &&
          chat.data.user.bio === `resume-${run}-24` &&
          window.__syncTiming.seq >= cursor
        )
      },
      { cursor, run },
    )
    const resumeMs = await page.evaluate((start) => performance.now() - start, resumeStart)
    if (run)
      samples.push({
        workspaceMs: timings.workspace,
        messagesMs: timings.messages,
        connectedMs: timings.connected,
        allHistoryMs,
        resumeMs,
      })
  }
  const replayPage = await context.newPage()
  replayPage.on('pageerror', (error) => errors.push(error.message))
  await replayPage.goto(ui)
  await replayPage.waitForFunction(
    () =>
      document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
        ?.status === 'connected',
  )
  const markedUnread = await context.request.patch(`${ui}/api/v1/conversations/sync-room-0`, {
    data: { unreadFrom: 'sync-room-0-message-199' },
  })
  assert.equal(markedUnread.status(), 200)
  await replayPage.waitForFunction(() => {
    const conversation = document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .data.conversations.find((item) => item.id === 'sync-room-0')
    return conversation.unreadCount > 0 && conversation.unreadFrom === 'sync-room-0-message-199'
  })
  const unreadBeforeReplay = await replayPage.evaluate(
    () =>
      document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .data.conversations.find((conversation) => conversation.id === 'sync-room-0').unreadCount,
  )
  assert(unreadBeforeReplay > 0, 'fixture must start replay with unread messages')
  await replayPage.evaluate(() => {
    window.__syncEvents = []
  })
  await context.setOffline(true)
  await replayPage.waitForFunction(() =>
    window.__syncSockets.every((socket) => socket.readyState === WebSocket.CLOSED),
  )
  const replayMutation = await fetch(`${control}?run=replay&replayMessage=true`, { method: 'POST' })
  assert.equal(replayMutation.status, 200)
  const { cursor: replayCursor, replaySeq } = await replayMutation.json()
  await context.setOffline(false)
  await replayPage.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .reconnect(),
  )
  await replayPage.waitForFunction(
    ({ replayCursor }) => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      return chat?.status === 'connected' && chat.cursor >= replayCursor
    },
    { replayCursor },
  )
  await replayPage.getByText('Replay fixture message', { exact: true }).waitFor()
  const replayFrames = await replayPage.evaluate(() => window.__syncEvents)
  const messageIndex = replayFrames.findIndex(
    (event) =>
      event.type === 'message.created' &&
      event.data.id === 'sync-replayed-message' &&
      event.seq === replaySeq,
  )
  const barrierIndex = replayFrames.findIndex(
    (event) => event.type === 'session.synced' && event.cursor === replayCursor,
  )
  assert(
    messageIndex >= 0 && barrierIndex > messageIndex,
    'real replay message must precede the sync barrier',
  )
  assert.equal(
    await replayPage.evaluate(
      () =>
        document
          .querySelector('#__nuxt')
          .__vue_app__.config.globalProperties.$pinia._s.get('chat')
          .data.conversations.find((conversation) => conversation.id === 'sync-room-0').unreadCount,
    ),
    unreadBeforeReplay,
    'replay must preserve the server unread count',
  )
  assert.equal(
    await replayPage.locator('.toast').count(),
    0,
    'successful replay has no summary toast',
  )
  await replayPage.close()
  const cachedSamples = []
  if (label !== 'before') {
    // Persist the exact applied position, then restore on a narrow mobile view.
    await page.setViewportSize({ width: 390, height: 844 })
    const previousRequests = bootstrapRequests
    for (let run = 0; run < 10; run++) {
      await page.evaluate(() => {
        const chat = document
          .querySelector('#__nuxt')
          .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        chat.saveCheckpoint([])
        const checkpoint = JSON.parse(
          localStorage.getItem(
            `ircord.snapshot.${document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('auth').account.id}`,
          ),
        )
        if (checkpoint.bootstrap.cursor !== chat.cursor)
          throw new Error('Checkpoint cursor differs from applied state')
      })
      await page.goto(`${ui}?checkpoint=1`)
      await page.waitForFunction(() => {
        const t = window.__syncTiming
        return t.workspace && t.messages && t.connected
      })
      cachedSamples.push(await page.evaluate(() => ({ ...window.__syncTiming })))
    }
    assert.equal(bootstrapRequests, previousRequests, 'cached resume must not fetch bootstrap')
    assert(
      await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      'mobile layout fits',
    )
    await mkdir('../.data/sync-perf', { recursive: true })
    await page.screenshot({ path: '../.data/sync-perf/mobile-resumed.png' })

    // Hold an old history response while valid live edits, reactions, deletion,
    // and a new message arrive through the actual browser socket handlers.
    let releaseHistory
    let responseReady
    const held = new Promise((resolve) => {
      responseReady = resolve
    })
    const release = new Promise((resolve) => {
      releaseHistory = resolve
    })
    await page.route('**/conversations/sync-room-0/messages?*', async (route) => {
      const response = await route.fetch()
      const history = await response.json()
      responseReady(history)
      await release
      await route.fulfill({ response, json: history })
    })
    await page.evaluate(() => {
      void document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .resync()
    })
    const oldHistory = await held
    await page.waitForFunction(
      () =>
        document
          .querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')?.status === 'connected',
    )
    await page.evaluate((history) => {
      const chat = document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      const socket = window.__syncSockets.at(-1)
      let seq = chat.cursor
      const dispatch = (type, data) =>
        socket.dispatchEvent(
          new MessageEvent('message', { data: JSON.stringify({ v: 1, type, seq: ++seq, data }) }),
        )
      const [deleted, edited, reacted] = history.items.slice(-3)
      dispatch('message.updated', { ...deleted, deleted: true, text: '' })
      dispatch('message.updated', { ...edited, text: 'Edited while history was loading' })
      dispatch('message.updated', {
        ...reacted,
        reactions: [{ emoji: '👍', memberIds: [reacted.senderId] }],
      })
      dispatch('message.created', {
        ...reacted,
        id: 'sync-concurrent-message',
        text: 'Arrived while history was loading',
        createdAt: new Date().toISOString(),
      })
    }, oldHistory)
    releaseHistory()
    await page.getByText('Edited while history was loading', { exact: true }).waitFor()
    await page.getByText('Arrived while history was loading', { exact: true }).waitFor()
    assert.equal(
      await page.getByText(oldHistory.items.at(-3).text, { exact: true }).count(),
      0,
      'old history must not restore deleted text',
    )
    await page.unroute('**/conversations/sync-room-0/messages?*')

    // An expired cursor must replace state and history, then establish a new
    // socket. It must neither loop nor leave the UI falsely connected.
    await page.goto(ui)
    await page.waitForFunction(
      () =>
        document
          .querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')?.status === 'connected',
    )
    await page.evaluate(() => {
      const chat = document
        .querySelector('#__nuxt')
        ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
      chat.saveCheckpoint([])
      for (const socket of window.__syncSockets) socket.close()
    })
    const expired = await fetch(`${control}?run=expired&expire=true`, { method: 'POST' })
    assert.equal(expired.status, 200)
    const beforeReplacement = bootstrapRequests
    await page.goto(`${ui}?checkpoint=1`)
    await page.waitForFunction(() => {
      const chat = document
        .querySelector('#__nuxt')
        ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
      return chat?.status === 'connected' && chat.data.user.bio === 'resume-expired-24'
    })
    await page.getByText('Fixture message 199 in sync-room-0', { exact: true }).waitFor()
    assert.equal(
      bootstrapRequests - beforeReplacement,
      1,
      'expired replay requires one replacement snapshot',
    )

    // A read is optimistic in the UI. Suspending before its response must keep
    // the last confirmed checkpoint, including if the request later fails.
    await page.evaluate(() =>
      document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .saveCheckpoint([]),
    )
    const savedBeforeRead = await page.evaluate(
      (owner) => localStorage.getItem(`ircord.snapshot.${owner}`),
      owner,
    )
    let releaseRead
    const readRelease = new Promise((resolve) => {
      releaseRead = resolve
    })
    await page.route('**/api/v1/conversations/sync-room-0', async (route) => {
      await readRelease
      await route.fulfill({
        status: 503,
        contentType: 'application/json',
        body: JSON.stringify({ detail: 'Disposable read failure' }),
      })
    })
    await page.evaluate(() => {
      void document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .markRead('sync-room-0')
    })
    await page.waitForFunction(
      () =>
        !document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
          .checkpointReady,
    )
    await page.evaluate(() =>
      document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('chat')
        .saveCheckpoint([]),
    )
    assert.equal(
      await page.evaluate((owner) => localStorage.getItem(`ircord.snapshot.${owner}`), owner),
      savedBeforeRead,
    )
    releaseRead()
    await page.waitForFunction(
      () =>
        document.querySelector('#__nuxt').__vue_app__.config.globalProperties.$pinia._s.get('chat')
          .checkpointReady,
    )
    await page.unroute('**/api/v1/conversations/sync-room-0')

    const [logoutResponse] = await Promise.all([
      page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname === '/api/v1/auth/logout' &&
          response.request().method() === 'POST',
      ),
      page.waitForNavigation({ waitUntil: 'load' }),
      page.evaluate(() => {
        void document
          .querySelector('#__nuxt')
          .__vue_app__.config.globalProperties.$pinia._s.get('auth')
          .logout()
      }),
    ])
    assert.equal(logoutResponse.status(), 204, 'logout must succeed before the reload')
    await page.waitForFunction(
      () =>
        document
          .querySelector('#__nuxt')
          ?.__vue_app__?.config.globalProperties.$pinia?._s.get('auth')?.status === 'signed-out',
    )
    assert.equal(
      await page.evaluate((owner) => localStorage.getItem(`ircord.snapshot.${owner}`), owner),
      null,
      'page hiding must not recreate a signed-out checkpoint',
    )
    console.log(
      'Mobile checkpoints, zero-bootstrap resume, history/live overlap, expired-cursor recovery, pending reads, and sign-out cleanup pass.',
    )
  }
  assert.deepEqual(errors, [])
  const results = {
    label,
    workload: {
      channels: 3,
      members: 46,
      retainedMessages: 600,
      recentMessagesPerChannel: 40,
      replayChanges: 25,
      samples: samples.length,
    },
    samples,
    cachedSamples,
  }
  const median = (key) =>
    samples.map((s) => s[key]).sort((a, b) => a - b)[Math.floor(samples.length / 2)]
  results.median = Object.fromEntries(
    Object.keys(samples[0]).map((key) => [key, Math.round(median(key) * 100) / 100]),
  )
  await mkdir('../.data/sync-perf', { recursive: true })
  await writeFile(`../.data/sync-perf/${label}.json`, JSON.stringify(results, null, 2))
  console.log(JSON.stringify({ workload: results.workload, median: results.median }))
} catch (error) {
  if (serverLog) console.error(serverLog)
  throw error
} finally {
  await browser?.close()
  frontend.kill('SIGTERM')
}
