import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { build } from 'bun'
import { chromium } from 'playwright'

const activityPath = new URL('../app/composables/notifications/useActivity.ts', import.meta.url)
const source = await readFile(activityPath, 'utf8')
// Real Vue watchers and VueUse scope cleanup, with disposable stores and an HTTP recorder.
const fixture = `
import { createApp, h, nextTick, reactive, ref } from 'vue'
import { useActivity } from './app/composables/notifications/useActivity.ts'

window.startFixture = async ({ tabId, responseMode = 'ok' }) => {
  let focused = true
  let visibility = 'visible'
  let mounted = true
  const viewed = ref('conversation-a')
  const chat = reactive({
    routing: true, tabId, incarnation: 'page', routingSnapshot: { connectionId: 'socket' },
  })
  const notifications = reactive({ mode: 'open', permission: 'granted' })
  const reports = []
  const requests = new Set()
  const held = []
  const holdWaiters = []
  const timers = new Set()
  const listeners = []
  let trustedKeys = 0
  let trustedScrolls = 0
  window.addEventListener('keydown', (event) => { if (event.isTrusted) trustedKeys++ }, true)
  window.addEventListener('scroll', (event) => { if (event.isTrusted) trustedScrolls++ }, true)
  Object.defineProperty(document, 'hasFocus', { value: () => focused })
  Object.defineProperty(document, 'visibilityState', { get: () => visibility })

  // Instrument the installed browser clock; Date and timers advance together.
  const nativeSetTimeout = window.setTimeout.bind(window)
  const nativeClearTimeout = window.clearTimeout.bind(window)
  window.setTimeout = (callback, delay, ...args) => {
    const id = nativeSetTimeout(() => { timers.delete(id); callback(...args) }, delay)
    timers.add(id)
    return id
  }
  window.clearTimeout = (id) => { timers.delete(id); nativeClearTimeout(id) }
  for (const target of [window, document]) {
    const add = target.addEventListener.bind(target)
    const remove = target.removeEventListener.bind(target)
    const capture = (options) => typeof options === 'boolean' ? options : !!options?.capture
    target.addEventListener = (type, listener, options) => {
      if (!listeners.some((entry) => entry.target === target && entry.type === type &&
        entry.listener === listener && entry.capture === capture(options)))
        listeners.push({ target, type, listener, capture: capture(options) })
      add(type, listener, options)
    }
    target.removeEventListener = (type, listener, options) => {
      const index = listeners.findIndex((entry) => entry.target === target && entry.type === type &&
        entry.listener === listener && entry.capture === capture(options))
      if (index >= 0) listeners.splice(index, 1)
      remove(type, listener, options)
    }
  }
  const flushReports = async () => {
    await Promise.allSettled([...requests].filter((entry) => entry.mode !== 'hold').map((entry) => entry.promise))
    await Promise.resolve()
  }
  let app
  window.fixture = {
    chat, notifications, reports,
    api: {
      reportActivity: (body, keepalive) => {
        const mode = responseMode
        responseMode = 'ok'
        reports.push({ ...body, at: Date.now(), keepalive })
        const entry = { mode }
        entry.promise = (async () => {
          const response = await fetch('/api/v1/activity', {
            method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Fixture-Response': mode },
            body: JSON.stringify(body), keepalive,
          })
          if (!response.ok) throw new Error('Fixture request failed')
          if (mode === 'hold') await new Promise((resolve) => {
            held.push(resolve)
            holdWaiters.splice(0).forEach((ready) => ready())
          })
        })()
        requests.add(entry)
        void entry.promise.finally(() => requests.delete(entry)).catch(() => {})
        return entry.promise
      },
    },
    flushReports,
    waitForHeld: () => held.length ? Promise.resolve() : new Promise((resolve) => holdWaiters.push(resolve)),
    releaseHeld: async () => {
      held.splice(0).forEach((resolve) => resolve())
      await Promise.allSettled([...requests].map((entry) => entry.promise))
      await Promise.resolve()
    },
    setResponse: (mode) => { responseMode = mode },
    setViewed: (value) => { viewed.value = value; return nextTick() },
    setNotifications: (permission, mode = 'open') => {
      notifications.permission = permission
      notifications.mode = mode
      return nextTick()
    },
    setConnection: (connectionId) => {
      chat.routingSnapshot = connectionId ? { connectionId } : undefined
      return nextTick()
    },
    setRouting: (enabled) => { chat.routing = enabled; return nextTick() },
    setFocus: (value) => {
      focused = value
      window.dispatchEvent(new Event(value ? 'focus' : 'blur'))
      return nextTick()
    },
    setVisibility: (value) => {
      visibility = value
      document.dispatchEvent(new Event('visibilitychange'))
      return nextTick()
    },
    resources: () => ({ timers: timers.size, listeners: listeners.length, requests: requests.size,
      trustedKeys, trustedScrolls }),
    unmount: () => {
      if (mounted) { app.unmount(); mounted = false }
      return nextTick()
    },
  }
  app = createApp({ setup() { useActivity(() => viewed.value); return () => h('div', 'Activity fixture') } })
  app.mount('#app')
  await nextTick()
}
`
const bundle = await build({
  entrypoints: ['activity-fixture'],
  target: 'browser',
  define: {
    'process.env.NODE_ENV': '"production"',
    __VUE_OPTIONS_API__: 'true',
    __VUE_PROD_DEVTOOLS__: 'false',
  },
  plugins: [
    {
      name: 'activity-fixture',
      setup(builder) {
        builder.onResolve({ filter: /^activity-fixture$/ }, () => ({
          path: 'activity-fixture',
          namespace: 'activity-fixture',
        }))
        builder.onLoad({ filter: /.*/, namespace: 'activity-fixture' }, () => ({
          contents: fixture,
          loader: 'js',
          resolveDir: fileURLToPath(new URL('../', import.meta.url)),
        }))
        builder.onLoad({ filter: /\/useActivity\.ts$/ }, () => ({
          contents: `
import { watch, onBeforeUnmount } from 'vue'
import { useDocumentVisibility, useEventListener } from '@vueuse/core'
const useChatStore = () => window.fixture.chat
const useNotificationStore = () => window.fixture.notifications
const apiClient = () => window.fixture.api
${source}`,
          loader: 'ts',
          resolveDir: fileURLToPath(new URL('.', activityPath)),
        }))
      },
    },
  ],
})
if (!bundle.success) throw new AggregateError(bundle.logs, 'Activity fixture did not build')
const javascript = await bundle.outputs[0].text()
const httpReports = []
const server = createServer(async (request, response) => {
  if (request.url === '/api/v1/activity' && request.method === 'POST') {
    let body = ''
    for await (const chunk of request) body += chunk
    httpReports.push(JSON.parse(body))
    response.statusCode = request.headers['x-fixture-response'] === 'fail' ? 503 : 204
    response.end()
  } else if (request.url === '/fixture.js') {
    response.setHeader('Content-Type', 'text/javascript')
    response.end(javascript)
  } else {
    response.setHeader('Content-Type', 'text/html')
    response.end(
      '<!doctype html><title>Activity test</title><body style="height:3000px"><div id="app"></div><script type="module" src="/fixture.js"></script></body>',
    )
  }
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const browser = await chromium.launch({ headless: true })
const reports = (page) => page.evaluate(() => window.fixture.reports)
const flush = (page) => page.evaluate(() => window.fixture.flushReports())
const resources = (page) => page.evaluate(() => window.fixture.resources())
const advance = async (page, ms) => {
  await page.clock.runFor(ms)
  await flush(page)
}
const checkCadence = (items) => {
  const active = items.filter((report) => report.active)
  for (let index = 1; index < active.length; index++)
    assert.ok(
      active[index].at - active[index - 1].at >= 10_000,
      'Active reports exceeded one per 10 seconds',
    )
}
async function withFixture(name, check, options = {}) {
  const page = await browser.newPage()
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  const start = new Date('2026-01-01T00:00:00Z')
  await page.clock.install({ time: start })
  await page.clock.pauseAt(start)
  await page.goto(`http://127.0.0.1:${server.address().port}`)
  await page.evaluate((options) => window.startFixture(options), { tabId: name, ...options })
  await flush(page)
  try {
    await check(page)
    assert.deepEqual(errors, [], 'Browser errors')
  } finally {
    await page.evaluate(() => window.fixture.releaseHeld())
    await page.evaluate(() => window.fixture.unmount())
    await flush(page)
    const items = await reports(page)
    assert.equal(
      httpReports.filter((report) => report.tab === name).length,
      items.length,
      'Every attempt must be an isolated HTTP POST',
    )
    items.forEach((report, index) => {
      assert.equal(report.sequence, index + 1, 'Reports must remain ordered')
      assert.equal(report.tab, name)
      assert.equal(report.incarnation, 'page')
    })
    const remaining = await resources(page)
    assert.equal(remaining.timers, 0, 'Unmount left a timer')
    assert.equal(remaining.listeners, 0, 'Unmount left a scoped listener')
    await page.close()
  }
}

try {
  await withFixture('keyboard-burst', async (page) => {
    const initial = await reports(page)
    assert.equal(initial.length, 1)
    assert.equal(initial[0].signal, 'engagement')
    assert.equal(initial[0].viewedConversationId, 'conversation-a')
    const started = initial[0].at
    for (let second = 0; second < 34; second++) {
      // Playwright keyboard events are trusted browser input.
      await page.keyboard.type('abcde')
      await page.evaluate(async (second) => {
        // Routing snapshots change revision while retaining the same socket binding.
        await window.fixture.setConnection('socket')
        await window.fixture.setViewed('intermediate')
        await window.fixture.setViewed('conversation-' + second)
        await window.fixture.setNotifications(second % 2 ? 'denied' : 'granted')
      }, second)
      assert.ok((await resources(page)).timers <= 1, 'Input accumulated timer queues')
      await advance(page, 1_000)
    }
    let items = await reports(page)
    assert.deepEqual(
      items.map((report) => report.at - started),
      [0, 10_000, 20_000, 30_000],
    )
    assert.equal((await resources(page)).trustedKeys, 170)
    assert.ok(items.every((report) => report.signal === 'engagement'))
    assert.equal(items[1].viewedConversationId, 'conversation-9')
    assert.equal(items[1].permission, 'denied')
    assert.equal(items[1].canAlert, false)
    await page.evaluate(async () => {
      await window.fixture.setViewed('latest-pending-view')
      await window.fixture.setNotifications('granted', 'open')
    })
    await advance(page, 5_999)
    assert.equal((await reports(page)).length, 4)
    await advance(page, 1)
    items = await reports(page)
    assert.equal(items.length, 5)
    assert.equal(items.at(-1).viewedConversationId, 'latest-pending-view')
    assert.equal(items.at(-1).canAlert, true)
    assert.equal(items.at(-1).signal, 'engagement', 'Renewal displaced genuine engagement')
    checkCadence(items)
  })

  await withFixture('state-and-reading', async (page) => {
    await advance(page, 1_000)
    await page.evaluate(async () => {
      await window.fixture.setViewed('conversation-b')
      await window.fixture.setViewed(undefined)
      await window.fixture.setNotifications('denied')
      await window.fixture.setNotifications('granted', 'off')
    })
    await advance(page, 8_999)
    assert.equal((await reports(page)).length, 1, 'State watchers bypassed the shared cadence')
    await advance(page, 1)
    let items = await reports(page)
    assert.equal(items.at(-1).signal, 'renewal', 'A route change selected the focal device')
    assert.equal(items.at(-1).viewedConversationId, undefined)
    assert.equal(items.at(-1).canAlert, false)
    assert.equal(items.at(-1).permission, 'granted')
    for (let renewal = 0; renewal < 5; renewal++) await advance(page, 25_000)
    items = await reports(page)
    assert.equal(items.length, 7)
    for (let index = 2; index < items.length; index++) {
      assert.equal(items[index].signal, 'renewal')
      assert.equal(items[index].active, true)
      assert.equal(items[index].at - items[index - 1].at, 25_000, 'Passive lease was not renewed')
    }
    checkCadence(items)
  })

  await withFixture('scroll-attribution', async (page) => {
    await advance(page, 9_000)
    await page.keyboard.press('a')
    await advance(page, 1_000)
    assert.equal((await reports(page)).at(-1).signal, 'engagement')
    await advance(page, 1_001)
    await page.evaluate(() => window.scrollTo(0, 300))
    // Let Chromium deliver its native trusted scroll; this is not a throttle delay.
    await page.waitForTimeout(50)
    assert.ok(
      (await resources(page)).trustedScrolls > 0,
      'The browser did not emit a trusted scroll',
    )
    await advance(page, 8_999)
    assert.equal(
      (await reports(page)).length,
      2,
      'Programmatic scrolling extended input attribution',
    )
    await advance(page, 15_000)
    assert.equal(
      (await reports(page)).at(-1).signal,
      'renewal',
      'Programmatic scrolling became engagement',
    )
    const before = (await reports(page)).length
    await page.evaluate(() => window.dispatchEvent(new KeyboardEvent('keydown', { key: 'a' })))
    await advance(page, 10_000)
    assert.equal((await reports(page)).length, before, 'Synthetic input became engagement')
    await page.mouse.click(50, 50)
    await page.mouse.wheel(0, 200)
    assert.equal((await reports(page)).length, before, 'Pointer input sent synchronously')
    await advance(page, 1)
    assert.equal((await reports(page)).at(-1).signal, 'engagement')
    checkCadence(await reports(page))
  })

  await withFixture('lifecycle', async (page) => {
    await advance(page, 1_000)
    await page.keyboard.press('a')
    await page.evaluate(async () => {
      await window.fixture.setFocus(false)
      await window.fixture.setVisibility('hidden')
      window.dispatchEvent(new Event('pagehide'))
      await window.fixture.setViewed('background-view')
      await window.fixture.setNotifications('denied')
    })
    await flush(page)
    let items = await reports(page)
    assert.equal(items.length, 2, 'One leave sent duplicate releases')
    assert.equal(items.at(-1).signal, 'inactive')
    assert.equal(items.at(-1).active, false)
    assert.equal(items.at(-1).viewedConversationId, undefined)
    assert.equal(items.at(-1).keepalive, true)
    assert.equal((await resources(page)).timers, 0)
    await advance(page, 30_000)
    assert.equal((await reports(page)).length, 2, 'Leaving did not cancel pending active work')
    await page.evaluate(async () => {
      await window.fixture.setVisibility('visible')
      await window.fixture.setFocus(true)
    })
    await advance(page, 1)
    assert.equal((await reports(page)).length, 2, 'A hidden page resumed before pageshow')
    await page.evaluate(() => window.dispatchEvent(new Event('pageshow')))
    await advance(page, 1)
    items = await reports(page)
    assert.equal(items.at(-1).signal, 'engagement', 'Pageshow must re-establish the released lease')
    assert.equal(items.at(-1).viewedConversationId, 'background-view')
    const resumedAt = items.at(-1).at
    await page.keyboard.press('b')
    await advance(page, 1_000)
    await page.evaluate(() => window.fixture.setFocus(false))
    await flush(page)
    await advance(page, 1_000)
    await page.evaluate(() => window.fixture.setFocus(true))
    await advance(page, 7_998)
    assert.equal((await reports(page)).filter((report) => report.active).length, 2)
    await advance(page, 1)
    items = await reports(page)
    assert.equal(items.at(-1).at - resumedAt, 10_000, 'Refocus bypassed the active throttle')
    assert.equal(items.at(-1).signal, 'engagement')
    await page.keyboard.press('c')
    await page.evaluate(() => window.fixture.unmount())
    await flush(page)
    const afterUnmount = (await reports(page)).length
    assert.equal((await reports(page)).at(-1).signal, 'inactive')
    assert.equal((await resources(page)).timers, 0)
    assert.equal((await resources(page)).listeners, 0)
    await page.evaluate(async () => {
      await window.fixture.setViewed('disposed-view')
      await window.fixture.setConnection('disposed-socket')
      await window.fixture.setFocus(true)
    })
    await page.keyboard.press('d')
    await advance(page, 100_000)
    assert.equal((await reports(page)).length, afterUnmount, 'Disposed work reported activity')
    checkCadence(items)
  })

  await withFixture('visibility-and-pagehide', async (page) => {
    await advance(page, 1_000)
    await page.keyboard.press('a')
    await page.evaluate(() => window.fixture.setVisibility('hidden'))
    await flush(page)
    assert.equal((await reports(page)).at(-1).signal, 'inactive')
    assert.equal((await resources(page)).timers, 0)
    await advance(page, 2_000)
    assert.equal((await reports(page)).length, 2, 'Visibility did not cancel pending engagement')
    await page.evaluate(() => window.fixture.setVisibility('visible'))
    await advance(page, 6_999)
    assert.equal((await reports(page)).length, 2, 'Foreground return bypassed the active throttle')
    await advance(page, 1)
    assert.equal(
      (await reports(page)).at(-1).signal,
      'engagement',
      'Foreground return must re-establish the released lease without a focus event',
    )
    await page.keyboard.press('b')
    await page.evaluate(() => window.dispatchEvent(new Event('pagehide')))
    await flush(page)
    assert.equal((await reports(page)).at(-1).signal, 'inactive')
    await page.evaluate(() => window.fixture.setViewed('pagehide-view'))
    await advance(page, 30_000)
    assert.equal(
      (await reports(page)).length,
      4,
      'Pagehide allowed stale active work while still focused',
    )
    assert.equal((await resources(page)).timers, 0)
    await page.evaluate(() => window.dispatchEvent(new Event('pageshow')))
    await advance(page, 1)
    assert.equal((await reports(page)).length, 5)
    assert.equal(
      (await reports(page)).at(-1).signal,
      'engagement',
      'Pageshow did not reactivate the lease',
    )
    assert.equal((await reports(page)).at(-1).viewedConversationId, 'pagehide-view')
    await advance(page, 25_000)
    assert.equal(
      (await reports(page)).at(-1).signal,
      'renewal',
      'Passive reading became fresh engagement',
    )
    checkCadence(await reports(page))
    await page.evaluate(() => window.dispatchEvent(new Event('pagehide')))
    await flush(page)
    assert.equal((await reports(page)).length, 7)
    await page.evaluate(() => window.fixture.unmount())
    await flush(page)
    assert.equal((await reports(page)).length, 7, 'Unmount duplicated the pagehide release')
  })

  await withFixture('reconnection', async (page) => {
    await advance(page, 1_000)
    await page.keyboard.press('a')
    await page.evaluate(async () => {
      await window.fixture.setConnection(undefined)
      await window.fixture.setViewed('reconnected-view')
      await window.fixture.setNotifications('denied')
      await window.fixture.setConnection('new-socket')
    })
    await flush(page)
    let items = await reports(page)
    assert.equal(items.length, 2)
    assert.equal(items.at(-1).connectionId, 'new-socket')
    assert.equal(items.at(-1).signal, 'engagement')
    assert.equal(items.at(-1).viewedConversationId, 'reconnected-view')
    assert.equal(items.at(-1).permission, 'denied')
    await advance(page, 9_000)
    assert.equal((await reports(page)).length, 2, 'An old binding left a scheduled callback')
    await page.evaluate(() => window.fixture.setConnection('third-socket'))
    await flush(page)
    items = await reports(page)
    assert.equal(
      items.at(-1).signal,
      'renewal',
      'Rebinding without input selected the focal device',
    )
    assert.equal(items.at(-1).connectionId, 'third-socket')
    await advance(page, 1_000)
    assert.equal((await reports(page)).length, 3)
    await page.evaluate(() => window.fixture.setRouting(false))
    assert.equal((await resources(page)).timers, 0)
    await advance(page, 30_000)
    assert.equal((await reports(page)).length, 3, 'Disabled routing still reported')
    await page.evaluate(() => window.fixture.setRouting(true))
    await flush(page)
    assert.equal((await reports(page)).at(-1).signal, 'renewal')
  })

  await withFixture('failed-request', async (page) => {
    await advance(page, 1_000)
    await page.keyboard.press('a')
    await page.evaluate(() => window.fixture.setResponse('fail'))
    await advance(page, 9_000)
    assert.equal((await reports(page)).length, 2)
    await page.evaluate(async () => {
      await window.fixture.setViewed('retry-latest-view')
      await window.fixture.setNotifications('denied')
    })
    for (let second = 0; second < 9; second++) {
      await page.keyboard.press('b')
      await advance(page, 1_000)
      assert.equal((await reports(page)).length, 2, 'A failed request caused an early retry')
    }
    await advance(page, 1_000)
    const items = await reports(page)
    assert.equal(items.length, 3)
    assert.equal(items.at(-1).signal, 'engagement', 'Failure lost genuine engagement')
    assert.equal(items.at(-1).viewedConversationId, 'retry-latest-view')
    assert.equal(items.at(-1).permission, 'denied')
    checkCadence(items)
  })

  await withFixture(
    'pending-request',
    async (page) => {
      await page.evaluate(() => window.fixture.waitForHeld())
      await advance(page, 1_000)
      await page.keyboard.press('a')
      await page.evaluate(() => window.fixture.setViewed('pending-latest-view'))
      await advance(page, 20_000)
      assert.equal(
        (await reports(page)).length,
        1,
        'A pending request spawned parallel active requests',
      )
      assert.equal((await resources(page)).timers, 0)
      await page.evaluate(() => window.fixture.setConnection('replacement-socket'))
      await flush(page)
      let items = await reports(page)
      assert.equal(items.length, 2, 'A pending old binding blocked its replacement')
      assert.equal(items.at(-1).connectionId, 'replacement-socket')
      assert.equal(items.at(-1).viewedConversationId, 'pending-latest-view')
      await page.evaluate(() => window.fixture.releaseHeld())
      await advance(page, 10_000)
      assert.equal((await reports(page)).length, 2, 'An old completion revived stale active work')
      await page.keyboard.press('b')
      await advance(page, 1)
      items = await reports(page)
      assert.equal(items.at(-1).connectionId, 'replacement-socket')
      assert.equal(items.at(-1).signal, 'engagement')
    },
    { responseMode: 'hold' },
  )

  await withFixture(
    'release-pending-request',
    async (page) => {
      await page.evaluate(() => window.fixture.waitForHeld())
      await advance(page, 1_000)
      await page.keyboard.press('a')
      await page.evaluate(() => window.fixture.setFocus(false))
      await flush(page)
      assert.equal(
        (await reports(page)).length,
        2,
        'An in-flight active request blocked its release',
      )
      assert.equal((await reports(page)).at(-1).signal, 'inactive')
      await page.evaluate(() => window.fixture.releaseHeld())
      await advance(page, 30_000)
      assert.equal(
        (await reports(page)).length,
        2,
        'An old request completion revived activity after blur',
      )
      assert.equal((await resources(page)).timers, 0)
    },
    { responseMode: 'hold' },
  )

  const allowedFields = [
    'tab',
    'incarnation',
    'connectionId',
    'sequence',
    'signal',
    'active',
    'viewedConversationId',
    'formFactor',
    'canAlert',
    'permission',
  ]
  assert.ok(
    httpReports.every((report) => Object.keys(report).every((key) => allowedFields.includes(key))),
    'Activity transmitted input details',
  )
  console.log(
    'Notification activity: 34 seconds of trusted input, latest state, passive renewal, scroll attribution, lifecycle cleanup, reconnection and request failures passed',
  )
} finally {
  await browser.close()
  await new Promise((resolve) => server.close(resolve))
}
