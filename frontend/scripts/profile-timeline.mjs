// Read-only diagnostic in an isolated Firefox profile. Origin-filtered Orca cookies arrive on stdin,
// never in arguments, files, logs or the repository. All non-resolver HTTP mutations are blocked.
import { firefox } from 'playwright'
import { mkdir, mkdtemp, realpath, rm } from 'node:fs/promises'
import { join } from 'node:path'

const target = process.argv[process.argv.indexOf('--url') + 1]
if (!target || !/^http:\/\/(localhost|127\.0\.0\.1):3000\//.test(target))
  throw new Error('Pass a local frontend URL with --url')
const envelope = await (async () => {
  let input = ''
  for await (const chunk of process.stdin) input += chunk
  return JSON.parse(input)
})()
if (!envelope.ok || !Array.isArray(envelope.result?.cookies))
  throw new Error('Missing browser session')
const cookies = envelope.result.cookies.map(
  ({ name, value, domain, path, secure, httpOnly, sameSite }) => ({
    name,
    value,
    domain,
    path,
    secure,
    httpOnly,
    sameSite: ['Strict', 'Lax', 'None'].includes(sameSite) ? sameSite : 'Lax',
  }),
)
const temporary = await realpath(await mkdtemp('/tmp/ircord-timeline-profile-'))
let context
let stage = 'launch'
try {
  const appData = join(temporary, 'app')
  const localData = join(temporary, 'local')
  await Promise.all([mkdir(appData), mkdir(localData)])
  context = await firefox.launchPersistentContext(join(temporary, 'profile'), {
    headless: !process.argv.includes('--headed'),
    viewport: { width: 1100, height: 1200 },
    serviceWorkers: 'block',
    env: { ...process.env, MOZ_APP_DATA: appData, MOZ_LOCAL_APP_DATA: localData },
  })
  await context.addCookies(cookies)
  cookies.length = 0
  envelope.result.cookies = []
  await context.route('**/api/**', (route) => {
    const request = route.request()
    if (
      ['GET', 'HEAD', 'OPTIONS'].includes(request.method()) ||
      /\/link-media$/.test(new URL(request.url()).pathname)
    )
      return route.continue()
    return route.fulfill({ status: 403, contentType: 'application/json', body: '{}' })
  })
  const page = await context.newPage()
  const errorNames = []
  page.on('pageerror', (error) => errorNames.push(error.name))
  stage = 'open'
  await page.goto(target, { waitUntil: 'domcontentloaded' })
  await page.locator('.timeline-row').first().waitFor({ timeout: 20_000 })
  const results = []
  for (const containment of [true, false]) {
    stage = containment ? 'profile-default' : 'profile-without-row-skipping'
    if (!containment)
      await page.addStyleTag({
        content:
          '.timeline-row { content-visibility: visible !important; contain-intrinsic-size: none !important; }',
      })
    for (let i = 0; i < 2; i++) {
      await page.locator('.timeline-scroll').evaluate((element) => {
        element.scrollTop = 0
      })
      await page.waitForTimeout(450)
    }
    const bounds = await page.locator('.timeline-scroll').boundingBox()
    await page.mouse.move(bounds.x + bounds.width - 12, bounds.y + bounds.height / 2)
    await page.evaluate(() => {
      const start = performance.now()
      const trace = { frames: [], timers: [], previousFrame: start, previousTimer: start }
      const frame = (now) => {
        trace.frames.push(now - trace.previousFrame)
        trace.previousFrame = now
        trace.raf = requestAnimationFrame(frame)
      }
      trace.raf = requestAnimationFrame(frame)
      trace.timer = setInterval(() => {
        const now = performance.now()
        trace.timers.push(now - trace.previousTimer)
        trace.previousTimer = now
      }, 25)
      window.__timelineProfile = trace
    })
    for (let i = 0; i < 14; i++) {
      await page.mouse.wheel(0, 650)
      await page.waitForTimeout(80)
    }
    await page.waitForTimeout(300)
    results.push(
      await page.evaluate((containment) => {
        const trace = window.__timelineProfile
        cancelAnimationFrame(trace.raf)
        clearInterval(trace.timer)
        delete window.__timelineProfile
        const summarize = (values) => {
          const sorted = values.sort((a, b) => a - b)
          return {
            count: sorted.length,
            p95: Math.round(sorted[Math.floor(sorted.length * 0.95)] ?? 0),
            max: Math.round(sorted.at(-1) ?? 0),
          }
        }
        const scroller = document.querySelector('.timeline-scroll')
        const rows = [...document.querySelectorAll('.timeline-row')]
        return {
          containment,
          frames: summarize(trace.frames),
          timers: summarize(trace.timers),
          rows: rows.length,
          skippedRows: rows.filter((row) => getComputedStyle(row).contentVisibility === 'auto')
            .length,
          scrollTop: Math.round(scroller.scrollTop),
          height: scroller.scrollHeight,
          iframes: scroller.querySelectorAll('iframe').length,
        }
      }, containment),
    )
  }
  console.log(JSON.stringify({ browser: 'firefox', results, errorNames }))
} catch (error) {
  console.error(JSON.stringify({ failed: true, stage, name: error.name }))
  process.exitCode = 1
} finally {
  await context?.close()
  await rm(temporary, { recursive: true, force: true })
}
