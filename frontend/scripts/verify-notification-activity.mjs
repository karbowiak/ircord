import { createServer } from 'node:http'
import { readFile } from 'node:fs/promises'
import { Transpiler } from 'bun'
import { chromium } from 'playwright'

const source = await readFile(new URL('../app/composables/useActivity.ts', import.meta.url), 'utf8')
const activity = new Transpiler({ loader: 'ts', target: 'browser' }).transformSync(source)
const server = createServer((request, response) => {
  response.setHeader('Content-Type', 'text/javascript')
  if (request.url === '/app/composables/useActivity.js') response.end(activity)
  else if (request.url === '/shared/utils/activity')
    response.end("export const formFactor = () => 'desktop'")
  else {
    response.setHeader('Content-Type', 'text/html')
    response.end('<!doctype html><title>Activity test</title><body style="height:3000px"></body>')
  }
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const browser = await chromium.launch({ headless: true })
try {
  const page = await browser.newPage()
  await page.goto(`http://127.0.0.1:${server.address().port}`)
  await page.evaluate(async () => {
    let clock = 100_000
    let focused = true
    let viewed = 'conversation-a'
    const reports = []
    const intervals = []
    const watchers = []
    const chat = {
      routing: true,
      tabId: 'tab',
      incarnation: 'page',
      routingSnapshot: { connectionId: 'socket' },
    }
    const updateWatchers = () => {
      watchers.forEach(({ source, callback, value: oldValue }, index) => {
        const next = source()
        if (next !== oldValue) {
          watchers[index].value = next
          callback(next, oldValue)
        }
      })
    }
    Object.defineProperty(document, 'hasFocus', { value: () => focused })
    Date.now = () => clock
    Object.assign(window, {
      reports,
      advance: (ms) => {
        clock += ms
      },
      setFocus: (value) => {
        focused = value
      },
      setViewed: (value) => {
        viewed = value
        updateWatchers()
      },
      setConnection: (connectionId) => {
        chat.routingSnapshot = connectionId ? { connectionId } : undefined
        updateWatchers()
      },
      tick: () => intervals.forEach((callback) => callback()),
      useChatStore: () => chat,
      useNotificationStore: () => ({ mode: 'open', permission: 'granted' }),
      useDocumentVisibility: () => ({ value: 'visible' }),
      apiClient: () => ({
        reportActivity: async (body) => {
          reports.push(body)
        },
      }),
      useEventListener: (target, names, callback, options) => {
        for (const name of Array.isArray(names) ? names : [names])
          target.addEventListener(name, callback, options)
      },
      watch: (source, callback, options) => {
        if (typeof source === 'function') watchers.push({ source, callback, value: source() })
        if (options?.immediate) callback(source())
      },
      useIntervalFn: (callback) => intervals.push(callback),
      onBeforeUnmount: () => {},
    })
    const module = await import('/app/composables/useActivity.js')
    module.useActivity(() => viewed)
  })
  let reports = await page.evaluate(() => window.reports)
  if (
    reports.length !== 1 ||
    reports[0].signal !== 'engagement' ||
    reports[0].viewedConversationId !== 'conversation-a'
  )
    throw new Error('Initial focus was not reported as engagement')
  await page.evaluate(() => {
    window.advance(11 * 60_000)
    window.tick()
  })
  reports = await page.evaluate(() => window.reports)
  if (reports.at(-1).signal !== 'renewal' || !reports.at(-1).active)
    throw new Error('Passive reading did not renew its lease')
  await page.evaluate(() => window.scrollTo(0, 300))
  await page.waitForTimeout(50)
  reports = await page.evaluate(() => window.reports)
  if (reports.length !== 2) throw new Error('Programmatic scrolling was treated as engagement')
  await page.evaluate(() => {
    window.advance(2_000)
    window.setViewed('conversation-b')
  })
  reports = await page.evaluate(() => window.reports)
  if (reports.at(-1).signal !== 'renewal' || reports.at(-1).viewedConversationId !== 'conversation-b')
    throw new Error('A route change must update the view without selecting the focal device')
  await page.keyboard.press('a')
  reports = await page.evaluate(() => window.reports)
  if (
    reports.at(-1).signal !== 'engagement' ||
    reports.at(-1).viewedConversationId !== 'conversation-b'
  )
    throw new Error('Input did not engage the current conversation')
  const beforeBurst = reports.length
  await page.keyboard.press('b')
  await page.keyboard.press('c')
  reports = await page.evaluate(() => window.reports)
  if (reports.length !== beforeBurst) throw new Error('Input burst was not coalesced')
  await page.evaluate(() => {
    window.setConnection(undefined)
    window.setConnection('new-socket')
  })
  reports = await page.evaluate(() => window.reports)
  if (reports.at(-1).signal !== 'renewal' || reports.at(-1).connectionId !== 'new-socket')
    throw new Error('Reconnecting without input selected the focal device')
  await page.evaluate(() => {
    window.setFocus(false)
    window.dispatchEvent(new Event('blur'))
  })
  reports = await page.evaluate(() => window.reports)
  if (reports.at(-1).signal !== 'inactive' || reports.at(-1).active)
    throw new Error('Blur did not end activity')
  if (
    reports.some(
      (report, index) =>
        report.sequence !== index + 1 ||
        !['socket', 'new-socket'].includes(report.connectionId) ||
        report.incarnation !== 'page',
    )
  )
    throw new Error('Activity reports are not fenced and ordered')
  console.log('Notification activity: passive reading, engagement, blur and fencing passed')
} finally {
  await browser.close()
  await new Promise((resolve) => server.close(resolve))
}
