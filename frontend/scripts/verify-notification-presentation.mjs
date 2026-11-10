import { createServer } from 'node:http'
import { readFile } from 'node:fs/promises'
import { Transpiler } from 'bun'
import { chromium } from 'playwright'

const source = await readFile(new URL('../app/composables/useNotifications.ts', import.meta.url), 'utf8')
const notifications = new Transpiler({ loader: 'ts', target: 'browser' }).transformSync(source)
const server = createServer((request, response) => {
  response.setHeader('Content-Type', 'text/javascript')
  if (request.url === '/app/composables/useNotifications.js') response.end(notifications)
  else if (request.url === '/shared/utils/activity') response.end("export const alertText = () => ({ title: 'Title', body: 'Body' })")
  else if (request.url === '/app/utils/notification-ledger') response.end('export const claimNotification = key => window.claim(key)')
  else {
    response.setHeader('Content-Type', 'text/html')
    response.end('<!doctype html><title>Notification presentation test</title>')
  }
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const browser = await chromium.launch({ headless: true })
try {
  const page = await browser.newPage()
  await page.goto(`http://127.0.0.1:${server.address().port}`)
  await page.evaluate(async () => {
    let focused = true
    let visible = true
    let viewed = 'a'
    let listener
    let release
    let delay = false
    const claims = []
    const sounds = []
    const shown = []
    Object.defineProperty(document, 'hasFocus', { value: () => focused })
    Object.defineProperty(document, 'visibilityState', { get: () => visible ? 'visible' : 'hidden' })
    Object.assign(window, {
      claims, sounds, shown,
      state: (next) => {
        if ('focused' in next) focused = next.focused
        if ('visible' in next) visible = next.visible
        if ('viewed' in next) viewed = next.viewed
      },
      deferClaim: () => { delay = true },
      releaseClaim: () => release(),
      claim: async (key) => {
        claims.push(key)
        if (delay) {
          delay = false
          await new Promise((resolve) => { release = resolve })
        }
        return true
      },
      emitAlert: (overrides = {}) => listener({
        delivery: 'os', generation: '1', alertId: 'id', sound: true, preview: true,
        conversationId: 'a', kind: 'mention', ...overrides,
      }),
      useI18n: () => ({ t: (key) => key, n: (value) => value }),
      useChatStore: () => ({
        data: { conversations: [] }, onAlert: (callback) => { listener = callback; return () => {} },
      }),
      useNotificationStore: () => ({ mode: 'open', permission: 'granted' }),
      useAlertSound: () => ({ play: (kind) => sounds.push(kind) }),
      computed: (fn) => ({ get value() { return fn() } }),
      watch: () => {}, useEventListener: () => {}, onBeforeUnmount: () => {},
      Notification: class { constructor(title, options) { shown.push({ title, options }) } },
    })
    const module = await import('/app/composables/useNotifications.js')
    module.useNotifications(() => viewed)
  })
  await page.evaluate(async () => {
    await window.emitAlert({ delivery: 'sound' })
    await window.emitAlert()
    if (window.claims.length || window.sounds.length || window.shown.length)
      throw new Error('Viewed conversation interrupted or claimed an alert')
    window.state({ viewed: 'b' })
    await window.emitAlert({ alertId: 'other' })
    if (window.claims.join() !== '1:other:sound' || window.sounds.length !== 1 || window.shown.length)
      throw new Error('Focused OS alert did not become sound without claiming OS')
    window.state({ viewed: undefined }) // Search, Inbox, drawers and dialogs share this state.
    await window.emitAlert({ alertId: 'search', delivery: 'sound' })
    if (window.sounds.length !== 2) throw new Error('A non-conversation view suppressed sound')
    window.state({ focused: false, viewed: 'a' })
    await window.emitAlert({ alertId: 'inactive' })
    if (window.shown.length !== 1 || !window.claims.includes('1:inactive:os'))
      throw new Error('Inactive window did not show routed OS alert')
    window.deferClaim()
    const pending = window.emitAlert({ alertId: 'late' })
    while (!window.claims.includes('1:late:os')) await Promise.resolve()
    window.state({ focused: true, viewed: 'a' })
    window.releaseClaim()
    await pending
    if (window.shown.length !== 1 || window.sounds.length !== 2)
      throw new Error('Focus and view change during claim still interrupted')
    window.state({ visible: false })
    await window.emitAlert({ alertId: 'hidden', delivery: 'sound' })
    if (window.sounds.length !== 3) throw new Error('A hidden window falsely counted as viewing')
  })
  console.log('Notification presentation: focused, viewed, non-conversation and queued alert guards passed')
} finally {
  await browser.close()
  await new Promise((resolve) => server.close(resolve))
}
