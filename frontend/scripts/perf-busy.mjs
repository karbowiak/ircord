// Busy-channel performance run (#13). Needs a demo-mode server:
//   bun run build && NUXT_DEMO=true PORT=4174 bun run preview
//   bun scripts/perf-busy.mjs             (BASE=http://127.0.0.1:4174 by default)
// Load is configurable: MEMBERS, HISTORY, RATE, CHURN, SECONDS, plus FLOOD (seconds measured).
import { chromium } from 'playwright'

const base = process.env.BASE ?? 'http://127.0.0.1:4174'
const load = {
  members: Number(process.env.MEMBERS ?? 5000),
  history: Number(process.env.HISTORY ?? 5000),
  rate: Number(process.env.RATE ?? 20),
  churn: Number(process.env.CHURN ?? 10),
  seconds: Number(process.env.SECONDS ?? 120),
}
const flood = Number(process.env.FLOOD ?? 15)

const browser = await chromium.launch({ args: ['--enable-precise-memory-info'] })
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
const cdp = await page.context().newCDPSession(page)
await cdp.send('Performance.enable')
const errors = []
page.on('pageerror', (e) => errors.push(e.message))

async function metrics() {
  const { metrics } = await cdp.send('Performance.getMetrics')
  return Object.fromEntries(metrics.map((m) => [m.name, m.value]))
}
/** Collects frame times and long tasks in the page while `run` executes. */
async function measure(label, run) {
  await page.evaluate(() => {
    const w = window
    w.__perf = { frames: [], long: [] }
    let last = performance.now()
    const loop = (t) => {
      w.__perf.frames.push(t - last)
      last = t
      if (w.__perf) w.__perf.raf = requestAnimationFrame(loop)
    }
    w.__perf.raf = requestAnimationFrame(loop)
    w.__perf.observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) w.__perf.long.push(entry.duration)
    })
    w.__perf.observer.observe({ type: 'longtask' })
  })
  const before = await metrics()
  const started = Date.now()
  await run()
  const elapsed = (Date.now() - started) / 1000
  const after = await metrics()
  const sample = await page.evaluate(() => {
    const w = window
    cancelAnimationFrame(w.__perf.raf)
    w.__perf.observer.disconnect()
    const result = { frames: w.__perf.frames.slice(1), long: w.__perf.long }
    w.__perf = undefined
    return result
  })
  const frames = sample.frames.sort((a, b) => a - b)
  const at = (q) => frames[Math.min(frames.length - 1, Math.floor(q * frames.length))] ?? 0
  const delta = (key) => (after[key] - before[key]) * 1000
  const row = {
    scenario: label,
    seconds: +elapsed.toFixed(1),
    fps: +(frames.length / elapsed).toFixed(1),
    'frame p50': +at(0.5).toFixed(1),
    'frame p95': +at(0.95).toFixed(1),
    'frame max': +(frames.at(-1) ?? 0).toFixed(0),
    'frames >50ms': frames.filter((f) => f > 50).length,
    'long tasks': sample.long.length,
    'long ms': Math.round(sample.long.reduce((a, b) => a + b, 0)),
    'script ms': Math.round(delta('ScriptDuration')),
    'layout ms': Math.round(delta('LayoutDuration')),
    'style ms': Math.round(delta('RecalcStyleDuration')),
    'heap MB': +(after.JSHeapUsedSize / 1048576).toFixed(1),
    nodes: after.Nodes,
  }
  results.push(row)
  return row
}
const results = []

// ---- Sign in and create the busy channel ------------------------------------------------------
await page.goto(base)
await page.getByRole('button', { name: 'Create an account' }).click()
const username = `perf${Date.now().toString(36)}`
await page.getByLabel('Username', { exact: true }).fill(username)
await page.getByText(`${username} is available`).waitFor()
await page.getByLabel('Password', { exact: true }).fill('perf-test-password')
await page.getByRole('button', { name: 'Create account', exact: true }).click()
await page.getByRole('textbox', { name: 'Message #general' }).waitFor()
const started = await page.request.post(`${base}/api/v1/_demo/busy`, { data: load })
console.log('load', await started.json())
const busyLink = page.locator('.conversation-link').filter({ hasText: /^busy/ })
await busyLink.waitFor({ timeout: 30_000 })

// ---- Opening the channel -----------------------------------------------------------------------
let t = Date.now()
await busyLink.click()
await page.getByRole('textbox', { name: 'Message #busy' }).waitFor()
await page.locator('.message-row').first().waitFor()
const openMs = Date.now() - t

// ---- Sitting at the bottom while the flood arrives ---------------------------------------------
await measure(`flood ${load.rate}/s, churn ${load.churn}/s`, () =>
  page.waitForTimeout(flood * 1000),
)
const heapAfterFlood = (await metrics()).JSHeapUsedSize

// ---- Scrolling back through history -------------------------------------------------------------
await measure('scroll history', async () => {
  for (let i = 0; i < 40; i++) {
    await page.mouse.move(800, 400)
    await page.mouse.wheel(0, -3000)
    await page.waitForTimeout(120)
  }
})
const rows = await page.locator('.message-row').count()

// ---- Member list -------------------------------------------------------------------------------
await measure('scroll members', async () => {
  const box = await page.locator('.member-sidebar').boundingBox()
  if (!box) return
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2)
  for (let i = 0; i < 40; i++) {
    await page.mouse.wheel(0, 4000)
    await page.waitForTimeout(60)
  }
})
const memberRows = await page
  .locator('.member-sidebar .member-row, .member-sidebar [role="option"], .member-sidebar li')
  .count()

// ---- Switching away and back -------------------------------------------------------------------
await page
  .locator('.conversation-link')
  .filter({ hasText: /^general/ })
  .first()
  .click()
await page.getByRole('textbox', { name: 'Message #general' }).waitFor()
t = Date.now()
await busyLink.click()
await page.getByRole('textbox', { name: 'Message #busy' }).waitFor()
await page.locator('.message-row').first().waitFor()
const backMs = Date.now() - t

// ---- Idle after the flood (memory should settle) ------------------------------------------------
await page.request.post(`${base}/api/v1/_demo/busy`, { data: { stop: true } })
await cdp.send('HeapProfiler.collectGarbage').catch(() => {})
await page.waitForTimeout(1000)
const settled = await metrics()

console.table(results)
console.log({
  'open #busy ms': openMs,
  'switch back ms': backMs,
  'message rows in DOM': rows,
  'member rows in DOM': memberRows,
  'heap after flood MB': +(heapAfterFlood / 1048576).toFixed(1),
  'heap after GC MB': +(settled.JSHeapUsedSize / 1048576).toFixed(1),
  'DOM nodes': settled.Nodes,
  'page errors': errors,
})
await browser.close()
