import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

async function openSettings(page: Page) {
  // The keyboard shortcut works at every width; the sidebar button is a drawer away on phones.
  await page.keyboard.press('ControlOrMeta+Comma')
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await expect(settings).toBeVisible()
  return { settings, search: settings.getByRole('combobox', { name: 'Search settings' }) }
}

test('settings search ranks results, opens the exact control and never changes it', async ({
  page,
}) => {
  await signIn(page)
  const { settings, search } = await openSettings(page)
  const results = settings.getByRole('listbox', { name: 'Matching settings' })

  await search.fill('  AUTOPLAY ')
  await expect(settings.getByRole('status').filter({ hasText: /found/ })).toHaveText(
    /settings? found/,
  )
  const first = results.getByRole('option').first()
  await expect(first).toContainText('Autoplay GIFs and videos')
  await expect(first).toContainText('Notifications › Conversation')
  await expect(first).toHaveAttribute('aria-selected', 'true')

  const toggle = settings.getByRole('switch', { name: 'Autoplay GIFs and videos' })
  await search.press('Enter')
  const row = settings.locator('[data-setting="autoplay-media"]')
  await expect(row).toBeFocused()
  await expect(row).toHaveClass(/setting-found/)
  await expect(row).toBeInViewport()
  await expect(toggle).toHaveAttribute('aria-checked', 'false')
  await expect(search).toHaveValue('')
  await expect(
    settings.getByRole('navigation', { name: 'Settings' }).getByRole('button', {
      name: 'Notifications',
    }),
  ).toHaveAttribute('aria-current', 'page')

  // Several results, keyboard selection, and an exact label ahead of synonyms.
  await search.fill('font')
  await expect(results.getByRole('option').nth(0)).toContainText('Message font')
  await expect(results.getByRole('option').nth(1)).toContainText('Typeface')
  await search.press('ArrowDown')
  await expect(results.getByRole('option').nth(1)).toHaveAttribute('aria-selected', 'true')
  await search.press('Enter')
  await expect(settings.locator('[data-setting="interface-font"]')).toBeFocused()

  // Synonyms, an explained unavailable setting, and no matches.
  await search.fill('do not disturb')
  await expect(results.getByRole('option').first()).toContainText('Quiet hours')
  await search.fill('test notification')
  await expect(results.getByRole('option').first()).toContainText(
    'Available when this device uses push notifications.',
  )
  await results.getByRole('option').first().click()
  await expect(settings.locator('[data-setting="notifications-device"]')).toBeFocused()
  // A result in a section whose name it shares still lands on its row.
  await search.fill('your networks')
  await search.press('Enter')
  await expect(settings.locator('[data-setting="network-list"]')).toBeFocused()

  // Broad queries are capped and say so.
  await search.fill('a')
  await expect(results.getByRole('option')).toHaveCount(20)
  await expect(settings.getByRole('status').filter({ hasText: /Showing/ })).toHaveText(
    /^Showing 20 of \d+ settings$/,
  )
  await expect(settings.getByText('Keep typing to narrow them.', { exact: false })).toBeVisible()

  await search.fill('add an emote')
  await expect(results.getByRole('option', { name: /Add an emote/ })).toHaveCount(0)
  await search.fill('qwertyuiop')
  await expect(
    settings.getByText('No settings match “qwertyuiop”.', { exact: false }),
  ).toBeVisible()

  // Escape clears first and keeps Settings open; the clear button does the same.
  await search.press('Escape')
  await expect(search).toHaveValue('')
  await expect(settings).toBeVisible()
  await search.fill('sound')
  await settings.getByRole('button', { name: 'Clear search' }).click()
  await expect(search).toHaveValue('')
  await expect(search).toBeFocused()
  await expect(settings.getByRole('heading', { name: 'Networks' })).toBeVisible()
  await search.press('Escape')
  await expect(settings).toBeHidden()
})

test('moving between results keeps what you typed in an unsaved form', async ({ page }) => {
  await signIn(page)
  const { settings, search } = await openSettings(page)
  await search.fill('display name')
  await search.press('Enter')
  const name = settings.getByLabel('Display name')
  await name.fill('Half typed')
  await search.fill('current password')
  await search.press('Enter')
  await settings.getByLabel('Current password').fill('not saved yet')
  await search.fill('bio')
  await search.press('Enter')
  await expect(settings.locator('[data-setting="about-you"]')).toBeFocused()
  await expect(name).toHaveValue('Half typed')
  await search.fill('password')
  await search.press('Enter')
  await expect(settings.getByLabel('Current password')).toHaveValue('not saved yet')
})

test('settings search works on a narrow screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 780 })
  await signIn(page)
  const { settings, search } = await openSettings(page)
  await expect(search).toBeInViewport()
  // One column: search above the section row, the panel below both at full width.
  const box = async (selector: string) => (await settings.locator(selector).boundingBox())!
  const field = await box('.settings-search')
  const nav = await box('.settings-nav')
  const panel = await box('.settings-content')
  expect(nav.y).toBeGreaterThanOrEqual(field.y + field.height - 1)
  expect(panel.y).toBeGreaterThanOrEqual(nav.y + nav.height - 1)
  expect(panel.width).toBeGreaterThan(300)
  await search.fill('quiet hours')
  const listed = await box('.settings-results')
  expect(listed.y).toBeGreaterThanOrEqual(nav.y + nav.height - 1)
  expect(listed.width).toBeGreaterThan(300)
  await settings.getByRole('option').first().click()
  await expect(settings.locator('[data-setting="quiet-hours"]')).toBeFocused()
  await expect(settings.locator('[data-setting="quiet-hours"]')).toBeInViewport()
})
