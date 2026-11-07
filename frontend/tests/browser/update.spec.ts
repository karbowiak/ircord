import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

// Kept in step with HIDDEN_GRACE_MS in app/composables/useAppUpdate.ts.
const PAST_GRACE = '02:05'
const MANIFEST = '**/_nuxt/builds/latest.json*'

const banner = (page: Page) => page.getByRole('status').filter({ hasText: 'IRCord was updated' })
const composer = (page: Page) => page.getByRole('combobox', { name: 'Message #general' })
const liveBuildId = (page: Page) =>
  page.evaluate(
    () =>
      (window as { __NUXT__?: { config?: { app?: { buildId?: string } } } }).__NUXT__?.config?.app
        ?.buildId ?? '',
  )

/** Counts full page loads from now on; any count above zero means the tab reloaded. */
function countLoads(page: Page) {
  const loads = { count: 0 }
  page.on('load', () => loads.count++)
  return loads
}

/** Answers the build manifest as if the server now ran `id`, then drops and restores the socket. */
async function deploy(page: Page, id: string) {
  await page.route(MANIFEST, (route) => route.fulfill({ json: { id, timestamp: Date.now() } }))
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
}

/** Makes the page report itself as a background tab, as switching tabs would. */
const hide = (page: Page) =>
  page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' })
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
    document.dispatchEvent(new Event('visibilitychange'))
  })

test.beforeEach(async ({ page }) => {
  await page.clock.install()
  await signIn(page)
})

test('a visible tab never reloads by itself, even long after the update', async ({ page }) => {
  const loads = countLoads(page)
  await deploy(page, 'newer-build')
  await expect(banner(page)).toBeVisible()
  await page.clock.fastForward('10:00')
  await page.waitForTimeout(500)
  expect(loads.count).toBe(0)
  await expect(banner(page)).toBeVisible()
})

test('Reload now loads the new build and keeps text typed just before the click', async ({
  page,
}) => {
  await deploy(page, 'newer-build')
  await expect(banner(page)).toBeVisible()
  await page.unroute(MANIFEST)
  // Typed and clicked within the drafts' 200 ms save debounce, so only the flush keeps it.
  await composer(page).fill('typed right before reloading')
  await Promise.all([
    page.waitForEvent('load'),
    banner(page).getByRole('button', { name: 'Reload now' }).click(),
  ])
  await expect(composer(page)).toHaveValue('typed right before reloading')
  await expect(banner(page)).toHaveCount(0)
})

test('the notice can be dismissed', async ({ page }) => {
  await deploy(page, 'newer-build')
  await banner(page).getByRole('button', { name: 'Later' }).click()
  await expect(banner(page)).toHaveCount(0)
})

test('a reconnect to the same build shows nothing and clears an earlier notice', async ({
  page,
}) => {
  const id = await liveBuildId(page)
  expect(id).not.toBe('')
  await deploy(page, 'newer-build')
  await expect(banner(page)).toBeVisible()
  await page.unroute(MANIFEST)
  // A rollback, or a replica that still serves this page's build.
  await deploy(page, id)
  await expect(banner(page)).toHaveCount(0)
})

test('a hidden, idle tab reloads to the new build once it has stayed hidden', async ({ page }) => {
  const loads = countLoads(page)
  await hide(page)
  await deploy(page, 'newer-build')
  await expect(banner(page)).toBeVisible()
  // A quick glance away is not enough.
  await page.waitForTimeout(500)
  expect(loads.count).toBe(0)
  await Promise.all([page.waitForEvent('load'), page.clock.fastForward(PAST_GRACE)])
})

/** Hidden past the grace period with `setup`'s state in place: the tab must not reload. */
async function expectNoSilentReload(page: Page) {
  const loads = countLoads(page)
  await hide(page)
  await deploy(page, 'newer-build')
  await expect(banner(page)).toBeVisible()
  await page.clock.fastForward(PAST_GRACE)
  await page.clock.fastForward('00:35') // one busy retry later
  await page.waitForTimeout(500)
  expect(loads.count).toBe(0)
}

test('a hidden tab keeps a queued upload', async ({ page }) => {
  await page.locator('input[type=file][multiple]').setInputFiles('public/avatars/noah.jpg')
  await expect(page.locator('.upload-item')).toHaveCount(1)
  await expectNoSilentReload(page)
  await expect(page.locator('.upload-item')).toHaveCount(1)
})

test('a hidden tab keeps an unacknowledged message', async ({ page }) => {
  // Demo sends travel over the socket; swallow them so the message is never acknowledged.
  await page.routeWebSocket(/\/api\/v1\/events/, (ws) => {
    const server = ws.connectToServer()
    ws.onMessage((frame) => {
      if (typeof frame === 'string' && frame.includes('"message.send"')) return
      server.send(frame)
    })
  })
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  await expect(page.locator('.connection-banner')).toHaveCount(0)
  await composer(page).fill('still sending')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(page.getByText('still sending')).toBeVisible()
  await expectNoSilentReload(page)
  await expect(page.getByText('still sending')).toBeVisible()
})

test('a hidden tab keeps a dialog with typed input', async ({ page }) => {
  await page.getByRole('button', { name: 'Add a network', exact: true }).click()
  await page.getByRole('textbox', { name: 'Hostname' }).fill('irc.example.org')
  await expectNoSilentReload(page)
  await expect(page.getByRole('textbox', { name: 'Hostname' })).toHaveValue('irc.example.org')
})

test('a hidden tab keeps typed text in a field outside any dialog', async ({ page }) => {
  await page
    .getByRole('complementary', { name: 'Conversations' })
    .getByRole('button', { name: /Server/ })
    .click()
  const filter = page.getByRole('textbox', { name: 'Filter server lines' })
  await filter.fill('MOTD')
  await expectNoSilentReload(page)
  await expect(filter).toHaveValue('MOTD')
})
