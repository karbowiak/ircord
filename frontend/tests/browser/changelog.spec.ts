import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

// The browser-test build is made with IRCORD_VERSION=v0.1.2 (playwright.config.ts), so the page
// runs v0.1.2. The routes below answer as if the server now ran a newer (or another) build.
const MANIFEST = '**/_nuxt/builds/latest.json*'
const CHANGELOG = '**/api/app/changelog*'
const PAST_GRACE = '02:05' // HIDDEN_GRACE_MS in useAppUpdate.ts, plus a little

const notes = {
  'v0.1.4':
    "## What's Changed\n* Add /gif <search terms> by @ppetermann in https://github.com/karbowiak/ircord/pull/67\n\n**Full Changelog**: https://github.com/karbowiak/ircord/compare/v0.1.3...v0.1.4",
  'v0.1.3':
    '## Highlights\n- Delete your own messages\n- [Unsafe link](javascript:alert(1)) <img src=x onerror=alert(1)>',
  'v0.1.2': 'Reload banner for new builds.',
  'v0.1.1': 'Read markers across devices.',
}
const releases = Object.entries(notes).map(([version, body]) => ({
  version,
  name: version,
  date: '2026-11-07T23:00:00Z',
  notes: body,
}))

const banner = (page: Page) => page.getByRole('status').filter({ hasText: 'IRCord was updated' })
const composer = (page: Page) => page.getByRole('combobox', { name: 'Message #general' })
const liveBuildId = (page: Page) =>
  page.evaluate(
    () =>
      (window as { __NUXT__?: { config?: { app?: { buildId?: string } } } }).__NUXT__?.config?.app
        ?.buildId ?? '',
  )
const dialog = (page: Page) => page.getByRole('dialog', { name: 'What’s new in IRCord' })

function countLoads(page: Page) {
  const loads = { count: 0 }
  page.on('load', () => loads.count++)
  return loads
}

/** The server now runs build `id` of `version`; its changelog answers with `body` when given. */
async function deploy(
  page: Page,
  id: string,
  version = 'v0.1.4',
  body?: (route: import('@playwright/test').Route) => Promise<void>,
) {
  await page.route(MANIFEST, (route) => route.fulfill({ json: { id, timestamp: Date.now() } }))
  await page.route(
    CHANGELOG,
    body ??
      ((route) =>
        route.fulfill({
          json: { buildId: id, version, releases: releases.filter((r) => r.version <= version) },
        })),
  )
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  await expect(banner(page)).toBeVisible()
}

test.beforeEach(async ({ page }) => {
  await page.clock.install()
  await signIn(page)
})

test('What’s new lists every skipped release and closes without reloading or losing the draft', async ({
  page,
}) => {
  const loads = countLoads(page)
  await composer(page).fill('half-written reply')
  await deploy(page, 'newer-build')
  const open = banner(page).getByRole('button', { name: 'What’s new' })
  await open.click()
  await expect(dialog(page)).toBeVisible()
  await expect(dialog(page).getByText('Reload to update from v0.1.2 to v0.1.4.')).toBeVisible()
  // Running v0.1.2: v0.1.3 and v0.1.4 are new, newest first; v0.1.2 itself is not repeated.
  await expect(dialog(page).getByRole('heading', { level: 3 })).toHaveText(['v0.1.4', 'v0.1.3'])
  await expect(dialog(page).getByRole('heading', { name: "What's Changed" })).toBeVisible()
  const pr = dialog(page).getByRole('link', { name: '#67' })
  await expect(pr).toHaveAttribute('href', 'https://github.com/karbowiak/ircord/pull/67')
  await expect(pr).toHaveAttribute('target', '_blank')
  await expect(dialog(page).getByRole('listitem').first()).toContainText('Add /gif <search terms>')
  // Markup and script links in notes stay text.
  await expect(dialog(page).getByRole('link', { name: 'Unsafe link' })).toHaveCount(0)
  await expect(dialog(page).getByText('<img src=x onerror=alert(1)>')).toBeVisible()
  await expect(dialog(page).locator('img')).toHaveCount(0)

  // Keyboard: Escape closes and focus returns to the button that opened it.
  await page.keyboard.press('Escape')
  await expect(dialog(page)).toHaveCount(0)
  await expect(open).toBeFocused()
  await open.press('Enter')
  await expect(dialog(page)).toBeVisible()
  await dialog(page).getByRole('button', { name: 'Close', exact: true }).click()
  await expect(dialog(page)).toHaveCount(0)

  expect(loads.count).toBe(0)
  await expect(composer(page)).toHaveValue('half-written reply')
  await expect(banner(page)).toBeVisible()
})

test('Reload now in the dialog reloads and keeps the draft', async ({ page }) => {
  await deploy(page, 'newer-build')
  // Stop the page's clock so the composer's 200 ms draft save never runs: only a save at reload
  // time keeps the text (the update's own flush, or the chat store's pagehide flush, either of
  // which suffices; removing both fails this test).
  await page.clock.pauseAt((await page.evaluate(() => Date.now())) + 1000)
  await composer(page).fill('typed before reading the notes')
  await banner(page).getByRole('button', { name: 'What’s new' }).click()
  await expect(dialog(page)).toBeVisible()
  await page.unroute(MANIFEST)
  await page.unroute(CHANGELOG)
  await Promise.all([
    page.waitForEvent('load'),
    dialog(page).getByRole('button', { name: 'Reload now' }).click(),
  ])
  await expect(composer(page)).toHaveValue('typed before reading the notes')
  await expect(dialog(page)).toHaveCount(0)
})

test('an open changelog keeps a hidden tab from reloading underneath it', async ({ page }) => {
  const loads = countLoads(page)
  await deploy(page, 'newer-build')
  await banner(page).getByRole('button', { name: 'What’s new' }).click()
  await expect(dialog(page)).toBeVisible()
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' })
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await page.clock.fastForward(PAST_GRACE)
  await page.clock.fastForward('00:35')
  await page.waitForTimeout(500)
  expect(loads.count).toBe(0)
  await expect(dialog(page)).toBeVisible()
})

test('unavailable or stale notes never block reloading', async ({ page }) => {
  await deploy(page, 'newer-build', 'v0.1.4', (route) => route.fulfill({ status: 500 }))
  await banner(page).getByRole('button', { name: 'What’s new' }).click()
  await expect(dialog(page).getByText('Couldn’t load the release notes.')).toBeVisible()

  // A replica that still serves another build answers: not presented as the update.
  await page.unroute(CHANGELOG)
  await page.route(CHANGELOG, (route) =>
    route.fulfill({ json: { buildId: 'some-other-build', version: 'v0.1.3', releases } }),
  )
  await dialog(page).getByRole('button', { name: 'Try again' }).click()
  await expect(dialog(page).getByText('The update is still rolling out')).toBeVisible()
  await expect(dialog(page).getByRole('heading', { level: 3 })).toHaveCount(0)

  // Data without notes for the deployed release.
  await page.unroute(CHANGELOG)
  await page.route(CHANGELOG, (route) =>
    route.fulfill({ json: { buildId: 'newer-build', version: '', releases: [] } }),
  )
  await dialog(page).getByRole('button', { name: 'Try again' }).click()
  await expect(
    dialog(page).getByText('Release notes aren’t available for this update.'),
  ).toBeVisible()

  await page.unroute(MANIFEST)
  await page.unroute(CHANGELOG)
  await Promise.all([
    page.waitForEvent('load'),
    dialog(page).getByRole('button', { name: 'Reload now' }).click(),
  ])
})

test('a rollback says so instead of offering older notes as new', async ({ page }) => {
  await deploy(page, 'rolled-back-build', 'v0.1.1')
  await banner(page).getByRole('button', { name: 'What’s new' }).click()
  await expect(
    dialog(page).getByText('IRCord was rolled back to v0.1.1. You’re running v0.1.2.'),
  ).toBeVisible()
  await expect(dialog(page).getByRole('heading', { level: 3 })).toHaveText(['v0.1.1'])
})

test('the dialog closes when the update is withdrawn and stays closed for the next one', async ({
  page,
}) => {
  const own = await liveBuildId(page)
  await deploy(page, 'newer-build')
  await banner(page).getByRole('button', { name: 'What’s new' }).click()
  await expect(dialog(page)).toBeVisible()
  // Rolled back to this page's own build: the notice and its dialog go away.
  await page.unroute(MANIFEST)
  await page.route(MANIFEST, (route) => route.fulfill({ json: { id: own, timestamp: Date.now() } }))
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  await expect(banner(page)).toHaveCount(0)
  await expect(dialog(page)).toHaveCount(0)
  // A later update shows the banner only; the dialog waits for a click.
  await page.unroute(MANIFEST)
  await page.unroute(CHANGELOG)
  await deploy(page, 'even-newer-build')
  await page.waitForTimeout(300)
  await expect(dialog(page)).toHaveCount(0)
})

async function openAbout(page: Page) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'About' })
    .click()
  return page.getByRole('region', { name: 'Release notes' })
}

test('Settings → About calls a rollback a rollback and notes the missing running release', async ({
  page,
}) => {
  await deploy(page, 'rolled-back-build', 'v0.1.1')
  const history = await openAbout(page)
  await expect(history.getByRole('status')).toHaveText(
    'You’re running IRCord v0.1.2. IRCord was rolled back to v0.1.1; reload to switch to it.',
  )
  await expect(history.getByRole('button', { name: 'Switch to v0.1.1' })).toBeVisible()
  await expect(history.getByRole('button', { name: 'Reload now' })).toHaveCount(0)
  await expect(history.getByText('The server has no notes for v0.1.2')).toBeVisible()
  await expect(
    history.getByRole('article', { name: 'v0.1.1', exact: true }).locator('.release-badge'),
  ).toHaveText('Available')
})

test('Settings → About does not offer an older replica’s build as an update', async ({ page }) => {
  // The changelog request reaches a replica still on another build; nothing announced it.
  await page.route(CHANGELOG, (route) =>
    route.fulfill({
      json: { buildId: 'older-replica', version: 'v0.1.1', releases: releases.slice(3) },
    }),
  )
  const history = await openAbout(page)
  await expect(history.getByRole('status')).toHaveText('You’re running IRCord v0.1.2.')
  await expect(history.getByRole('button')).toHaveCount(0)
  await expect(history.locator('.release-badge')).toHaveCount(0)
})

test('Settings → About keeps the release history after the banner is dismissed', async ({
  page,
}) => {
  await deploy(page, 'newer-build')
  await banner(page).getByRole('button', { name: 'Later' }).click()
  await expect(banner(page)).toHaveCount(0)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'About' })
    .click()
  const history = page.getByRole('region', { name: 'Release notes' })
  await expect(
    history.getByText('You’re running IRCord v0.1.2. v0.1.4 is available; reload to update.'),
  ).toBeVisible()
  const entry = (version: string) => history.getByRole('article', { name: version, exact: true })
  await expect(entry('v0.1.4').locator('.release-badge')).toHaveText('Available')
  await expect(entry('v0.1.3').locator('.release-badge')).toHaveText('Not installed yet')
  await expect(entry('v0.1.2').locator('.release-badge')).toHaveText('Running')
  await expect(entry('v0.1.1').locator('.release-badge')).toHaveCount(0)
  await expect(entry('v0.1.2')).toContainText('Reload banner for new builds.')
})

test('Settings → About on the current build shows its notes, or says when there are none', async ({
  page,
}) => {
  const buildId = await page.evaluate(
    () =>
      (window as { __NUXT__?: { config?: { app?: { buildId?: string } } } }).__NUXT__?.config?.app
        ?.buildId,
  )
  await page.route(CHANGELOG, (route) =>
    route.fulfill({ json: { buildId, version: 'v0.1.2', releases: releases.slice(2) } }),
  )
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'About' })
    .click()
  const history = page.getByRole('region', { name: 'Release notes' })
  await expect(history.getByRole('status')).toHaveText('You’re running IRCord v0.1.2.')
  await expect(history.getByRole('button', { name: 'Reload now' })).toHaveCount(0)
  await expect(history.getByRole('heading', { level: 5 })).toHaveText(['v0.1.2', 'v0.1.1'])

  // An unreleased build carries no notes.
  await page.unroute(CHANGELOG)
  await page.route(CHANGELOG, (route) =>
    route.fulfill({ json: { buildId, version: '', releases: [] } }),
  )
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'Shortcuts' })
    .click()
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'About' })
    .click()
  await expect(history.getByText('No release notes are available for this build.')).toBeVisible()
})

test('the banner and changelog fit a phone screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await deploy(page, 'newer-build')
  for (const name of ['What’s new', 'Reload now', 'Later'])
    await expect(banner(page).getByRole('button', { name })).toBeInViewport()
  await banner(page).getByRole('button', { name: 'What’s new' }).click()
  await expect(dialog(page)).toBeVisible()
  const reload = dialog(page).getByRole('button', { name: 'Reload now' })
  await expect(reload).toBeInViewport()
  expect(await dialog(page).evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
})
