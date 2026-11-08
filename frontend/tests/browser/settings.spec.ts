import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const violations = async (page: Page, scope = 'dialog') =>
  (
    await new AxeBuilder({ page })
      .include(scope)
      .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
      .analyze()
  ).violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }))
const openSettings = (page: Page) =>
  page.getByRole('button', { name: 'User settings', exact: true }).click()
const section = (page: Page, name: string) =>
  page.locator('.settings-nav').getByRole('button', { name, exact: true })
const content = (page: Page) => page.locator('.settings-content')

test('message and media controls live under Messages & media and keep their values', async ({
  page,
}) => {
  await openSettings(page)
  await section(page, 'Notifications').click()
  for (const name of ['Channel activity', 'Image and video previews', 'Autoplay GIFs and videos'])
    await expect(content(page).getByText(name, { exact: true })).toHaveCount(0)
  await expect(content(page).getByRole('switch', { name: 'Quiet hours' })).toBeVisible()

  await section(page, 'Messages & media').click()
  await expect(section(page, 'Messages & media')).toHaveAttribute('aria-current', 'page')
  await expect(page.locator('.settings-content > .settings-title')).toHaveText('Messages & media')
  const saved = page.waitForResponse(
    (r) => r.url().includes('/preferences') && r.request().method() === 'PUT' && r.ok(),
  )
  await content(page)
    .getByRole('radiogroup', { name: 'Join and part messages' })
    .getByRole('radio', { name: 'Hide' })
    .click()
  await saved

  // Read back from the account, not this device's copy.
  await page.evaluate(() => localStorage.removeItem('ircord.preferences'))
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await openSettings(page)
  await section(page, 'Messages & media').click()
  await expect(
    content(page)
      .getByRole('radiogroup', { name: 'Join and part messages' })
      .getByRole('radio', { name: 'Hide' }),
  ).toHaveAttribute('aria-checked', 'true')
  expect(await violations(page)).toEqual([])
})

test('ignored people moved with message visibility and still hide messages', async ({ page }) => {
  await openSettings(page)
  await section(page, 'Notifications').click()
  await expect(content(page).getByLabel('Ignore a nick or mask')).toHaveCount(0)
  // Notifications points to where ignoring lives now.
  await content(page).getByRole('button', { name: 'Messages & media' }).click()
  await expect(page.locator('.settings-content > .settings-title')).toBeFocused()
  await expect(section(page, 'Messages & media')).toHaveAttribute('aria-current', 'page')
  await content(page).getByLabel('Ignore a nick or mask').fill('Mika')
  await content(page).getByLabel('Ignore a nick or mask').press('Enter')
  await expect(content(page).getByRole('button', { name: 'Stop ignoring Mika' })).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(page.locator('.message-row').filter({ hasText: 'Coffee is on' })).toHaveCount(0)
})

test('reduce motion is explained where playback is set and overrides it', async ({ page }) => {
  await openSettings(page)
  await section(page, 'Messages & media').click()
  await section(page, 'Appearance').click()
  await expect(section(page, 'Appearance')).toHaveAttribute('aria-current', 'page')
  await content(page)
    .getByRole('radiogroup', { name: 'Reduce motion' })
    .getByRole('radio', { name: 'On' })
    .click()
  await expect(page.locator('html')).toHaveAttribute('data-motion', 'reduced')
  await content(page).getByRole('button', { name: 'Messages & media' }).first().click()
  await expect(content(page).locator('[data-setting="animate-media"]')).toContainText(
    'Reduce motion is on',
  )
})

test('each section resets only its own appearance settings', async ({ page }) => {
  await openSettings(page)
  await content(page).getByRole('radio', { name: 'Light', exact: true }).click()
  await section(page, 'Messages & media').click()
  await content(page)
    .getByRole('radio', { name: /^IRC One line/ })
    .click()
  await expect(page.locator('html')).toHaveAttribute('data-layout', 'irc')
  await content(page).getByRole('button', { name: 'Reset message layout' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-layout', 'cozy')
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await content(page)
    .getByRole('radio', { name: /^Compact Smaller/ })
    .click()
  await section(page, 'Appearance').click()
  await content(page).getByRole('button', { name: 'Reset appearance' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(page.locator('html')).toHaveAttribute('data-layout', 'compact')
})

test('selects work from the keyboard and their popup escapes the dialog', async ({ page }) => {
  await openSettings(page)
  const dark = content(page).getByRole('combobox', { name: 'Dark variant' })
  await expect(dark).toHaveAttribute('aria-expanded', 'false')
  const before = await dark.textContent()
  await dark.focus()
  await page.keyboard.press('ArrowDown')
  const list = page.getByRole('listbox')
  await expect(list).toBeVisible()
  // The popup is named by the same label as its button.
  await expect(list).toHaveAccessibleName('Dark variant')
  await expect(dark).toHaveAttribute('aria-expanded', 'true')
  // Escape closes without changing anything and keeps focus on the button.
  await page.keyboard.press('Escape')
  await expect(list).toBeHidden()
  await expect(dark).toBeFocused()
  await expect(dark).toHaveText(before!)

  await page.keyboard.press('Enter')
  await page.keyboard.press('End')
  const last = list.getByRole('option').last()
  const lastLabel = (await last.textContent())!.trim()
  await expect(dark).toHaveAttribute('aria-activedescendant', (await last.getAttribute('id'))!)
  await page.keyboard.press('Enter')
  await expect(list).toBeHidden()
  await expect(dark).toBeFocused()
  await expect(dark).toHaveText(lastLabel)

  // Type-ahead while closed picks the next match directly, like a native select.
  await page.keyboard.type('t')
  await expect(dark).toHaveText(/^Tokyo Night/)

  // The popup sits in the top layer, inside the viewport, and closes on an outside click.
  await dark.click()
  await expect(list).toBeVisible()
  const box = (await list.boundingBox())!
  const viewport = page.viewportSize()!
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height)
  expect(await list.evaluate((el) => el.matches(':popover-open'))).toBe(true)
  await expect(list.getByRole('option', { selected: true })).toHaveCount(1)
  await page.locator('.settings-content > .settings-title').click()
  await expect(list).toBeHidden()

  // The choice is saved and survives a reload.
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await openSettings(page)
  await expect(content(page).getByRole('combobox', { name: 'Dark variant' })).toHaveText(
    /^Tokyo Night/,
  )
  expect(await violations(page)).toEqual([])
})

test('segmented choices are radio groups that follow the arrow keys', async ({ page }) => {
  await openSettings(page)
  await section(page, 'Messages & media').click()
  const clock = content(page).getByRole('radiogroup', { name: 'Time format' })
  const h24 = clock.getByRole('radio', { name: '24-hour' })
  await expect(h24).toHaveAttribute('aria-checked', 'true')
  await expect(clock.getByRole('radio', { name: '12-hour' })).toHaveAttribute('tabindex', '-1')
  await h24.focus()
  await page.keyboard.press('ArrowRight')
  await expect(clock.getByRole('radio', { name: '12-hour' })).toBeFocused()
  await expect(clock.getByRole('radio', { name: '12-hour' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await expect(page.locator('.appearance-preview time').first()).toContainText(/AM|PM/i)
})

test('password fields keep their labels, help, errors and autofill hints', async ({ page }) => {
  await openSettings(page)
  await section(page, 'Account').click()
  const current = content(page).getByLabel('Current password')
  const next = content(page).getByLabel('New password')
  await expect(current).toHaveAttribute('autocomplete', 'current-password')
  await expect(next).toHaveAttribute('autocomplete', 'new-password')
  await expect(next).toHaveAccessibleDescription(/At least 8 characters/)
  await current.fill('not-my-password')
  await next.fill('another-password')
  await content(page).getByRole('button', { name: 'Change password' }).click()
  await expect(content(page).getByRole('alert')).toBeVisible()
  await expect(next).toHaveAttribute('aria-invalid', 'true')
  expect(await violations(page, '[data-setting="password"]')).toEqual([])
})

test('settings are grouped in the nav and open at the requested section', async ({ page }) => {
  await openSettings(page)
  const nav = page.getByRole('navigation', { name: 'Settings' })
  // Grouped for screen readers too, not only visually.
  for (const [group, first] of [
    ['User settings', 'Profile'],
    ['App settings', 'Appearance'],
    ['IRCord', 'Emotes'],
  ])
    await expect(
      nav.getByRole('group', { name: group }).getByRole('button', { name: first, exact: true }),
    ).toBeVisible()
  await expect(section(page, 'Appearance')).toHaveAttribute('aria-current', 'page')
  await page.keyboard.press('Escape')
  // The profile card's edit button opens Profile directly.
  await page.locator('.self-profile').click()
  await page.getByRole('button', { name: 'Edit profile' }).click()
  await expect(section(page, 'Profile')).toHaveAttribute('aria-current', 'page')
  await expect(page.locator('.settings-content > .settings-title')).toHaveText('Profile')
})

test('the narrow layout scrolls the section list instead of the page', async ({ page }) => {
  await openSettings(page)
  await page.setViewportSize({ width: 390, height: 844 })
  const headings = page.locator('.settings-nav-group')
  // Anchor on the rendered dialog first: the dialog loads lazily.
  await expect(headings).toHaveCount(3)
  // Visually hidden on a narrow screen, still named for screen readers.
  await expect(headings.first()).toHaveCSS('position', 'absolute')
  await expect(headings.first()).toHaveCSS('clip-path', 'inset(50%)')
  await expect(
    page.getByRole('navigation', { name: 'Settings' }).getByRole('group', { name: 'App settings' }),
  ).toHaveCount(1)
  const messages = section(page, 'Messages & media')
  await messages.scrollIntoViewIfNeeded()
  await messages.click()
  await expect(
    content(page).getByRole('switch', { name: 'Autoplay GIFs and videos' }),
  ).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(
    false,
  )
  for (const theme of ['Light', 'Dark']) {
    await section(page, 'Appearance').click()
    await content(page).getByRole('radio', { name: theme, exact: true }).click()
    await section(page, 'Messages & media').click()
    expect(await violations(page)).toEqual([])
  }
})
