// Isolated, signed-out fixture against the existing development frontend; no API mutations.
import { chromium, firefox } from 'playwright'
import { mkdir, mkdtemp, realpath, rm } from 'node:fs/promises'
import { join } from 'node:path'

const profile = await realpath(await mkdtemp('/tmp/ircord-scroll-check-'))
const browserType = process.argv.includes('--firefox') ? firefox : chromium
let context
try {
  const appData = join(profile, 'app-data')
  const localData = join(profile, 'local-data')
  await Promise.all([mkdir(appData), mkdir(localData)])
  context = await browserType.launchPersistentContext(profile, {
    headless: true,
    serviceWorkers: 'block',
    viewport: { width: 1100, height: 800 },
    // Firefox's profile flag does not isolate its application-data directory on macOS.
    env: { ...process.env, MOZ_APP_DATA: appData, MOZ_LOCAL_APP_DATA: localData },
  })
  await context.route('**/api/**', (route) =>
    ['GET', 'HEAD', 'OPTIONS'].includes(route.request().method())
      ? route.continue()
      : route.fulfill({ status: 403, body: '{}' }),
  )
  const page = await context.newPage()
  await page.goto('http://127.0.0.1:3000/', { waitUntil: 'domcontentloaded' })
  const modulePath = '/_nuxt/@fs' + new URL('./verify-embed-scroll.mjs', import.meta.url).pathname
  const result = await page.evaluate(
    async ({ modulePath, required }) => {
      const { measureEmbedScroll } = await import(modulePath)
      return measureEmbedScroll(required)
    },
    { modulePath, required: !process.argv.includes('--baseline') },
  )
  console.log(JSON.stringify({ browser: browserType.name(), ...result }))
} finally {
  await context?.close()
  await rm(profile, { recursive: true, force: true })
}
