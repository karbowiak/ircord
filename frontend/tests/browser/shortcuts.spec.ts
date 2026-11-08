import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

// The GIF picker loads Klipy categories when it opens; keep those requests local.
test.beforeEach(async ({ page }) => {
  await page.route('https://api.klipy.com/**', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ result: true, data: { categories: [] } }),
    }),
  )
  await signIn(page)
})

const composer = (page: Page) => page.getByRole('combobox', { name: 'Message #general' })
const picker = (page: Page) => page.getByRole('dialog', { name: 'GIF and emoji picker' })
const gifSearch = (page: Page) => page.getByRole('textbox', { name: 'Search Klipy GIFs' })
const help = (page: Page) => page.getByRole('dialog', { name: 'Keyboard shortcuts' })
const search = (page: Page) => page.getByRole('dialog', { name: 'Search' })
const caret = (page: Page) =>
  composer(page).evaluate((el: HTMLTextAreaElement) => [el.selectionStart, el.selectionEnd])
const blur = (page: Page) =>
  page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur())
/** Types a draft and puts the caret after “hello”, so restoring it is observable. */
async function draftWithCaret(page: Page) {
  const input = composer(page)
  await input.click()
  await input.fill('hello world')
  await input.evaluate((el: HTMLTextAreaElement) => el.setSelectionRange(5, 5))
  return input
}

for (const modifier of ['Control', 'Meta'] as const) {
  test(`${modifier}+G opens the GIF picker from the composer and Escape restores the draft`, async ({
    page,
  }) => {
    const rows = await page.locator('.message-row').count()
    const input = await draftWithCaret(page)
    await page.keyboard.press(`${modifier}+g`)
    await expect(picker(page)).toBeVisible()
    await expect(gifSearch(page)).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(picker(page)).toHaveCount(0)
    await expect(input).toBeFocused()
    await expect(input).toHaveValue('hello world')
    expect(await caret(page)).toEqual([5, 5])
    // The same key closes it again, also restoring the caret.
    await page.keyboard.press(`${modifier}+g`)
    await expect(gifSearch(page)).toBeFocused()
    await page.keyboard.press(`${modifier}+g`)
    await expect(picker(page)).toHaveCount(0)
    expect(await caret(page)).toEqual([5, 5])
    await expect(page.locator('.message-row')).toHaveCount(rows)
  })

  test(`${modifier}+K opens search from the composer and keeps the draft`, async ({ page }) => {
    const input = await draftWithCaret(page)
    await page.keyboard.press(`${modifier}+k`)
    await expect(search(page)).toBeVisible()
    await expect(page.getByRole('textbox', { name: 'Search workspace' })).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(search(page)).toHaveCount(0)
    await expect(input).toBeFocused()
    await expect(input).toHaveValue('hello world')
    expect(await caret(page)).toEqual([5, 5])
    // It toggles: pressed again inside search, it closes search.
    await page.keyboard.press(`${modifier}+k`)
    await expect(search(page)).toBeVisible()
    await page.keyboard.press(`${modifier}+k`)
    await expect(search(page)).toHaveCount(0)
    await expect(input).toBeFocused()
  })
}

test('Ctrl+G opens the GIF picker without focus in the composer', async ({ page }) => {
  await blur(page)
  await page.keyboard.press('Control+g')
  await expect(gifSearch(page)).toBeFocused()
  await page.keyboard.press('Escape')
  await expect(composer(page)).toBeFocused()
})

test('“?” is text in fields and opens help elsewhere; Ctrl+/ works everywhere', async ({
  page,
}) => {
  const input = composer(page)
  await input.click()
  await page.keyboard.type('why?')
  await expect(input).toHaveValue('why?')
  await expect(help(page)).toHaveCount(0)
  // Also in the GIF search field.
  await page.keyboard.press('Control+g')
  await expect(gifSearch(page)).toBeFocused()
  await page.keyboard.type('what?')
  await expect(gifSearch(page)).toHaveValue('what?')
  await expect(help(page)).toHaveCount(0)
  await page.keyboard.press('Escape')
  // Ctrl+/ opens help from the composer and returns focus there.
  await page.keyboard.press('Control+/')
  await expect(help(page)).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(help(page)).toHaveCount(0)
  await expect(input).toBeFocused()
  await expect(input).toHaveValue('why?')
  // Outside a field “?” opens it, and pressing it again closes it.
  await blur(page)
  await page.keyboard.press('?')
  await expect(help(page)).toBeVisible()
  await page.keyboard.press('?')
  await expect(help(page)).toHaveCount(0)
})

test('the help lists the working bindings with platform labels and is accessible', async ({
  page,
}) => {
  await blur(page)
  await page.keyboard.press('?')
  const dialog = help(page)
  await expect(dialog).toBeVisible()
  for (const [label, keys] of [
    ['GIF picker', 'Ctrl G'],
    ['Search and quick switcher', 'Ctrl K'],
    ['Settings', 'Ctrl ,'],
    ['Keyboard shortcuts (this list)', '? · Ctrl /'],
    ['Previous / next conversation', 'Alt ↑ / ↓'],
    ['Next unread or mention', 'Alt Shift ↑ / ↓'],
    ['Mark conversation as read', 'Esc'],
    ['Mark network as read', 'Shift Esc'],
    ['Bold', 'Ctrl B'],
    ['Mark as unread · delete (message toolbar)', 'Hold Shift'],
    ['New line', 'Shift Enter'],
    ['Context menu', 'Right-click · Shift F10 · long-press'],
  ])
    await expect(dialog.locator('dt', { hasText: label }).locator('+ dd')).toHaveText(keys)
  for (const group of ['Getting around', 'Reading', 'Writing', 'Formatting', 'App'])
    await expect(dialog.getByRole('heading', { name: group, exact: true })).toBeVisible()
  const violations = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
    .analyze()
  expect(violations.violations.map((v) => v.id)).toEqual([])
  await page.keyboard.press('Escape')
  // Controls carry the same hints, and Settings keeps the list.
  await expect(page.getByRole('button', { name: 'Choose a GIF', exact: true })).toHaveAttribute(
    'aria-keyshortcuts',
    'Control+G',
  )
  await expect(page.locator('.member-search kbd')).toHaveText('Ctrl K')
  // The message box names the GIF key to screen readers too.
  await expect(composer(page)).toHaveAttribute('aria-keyshortcuts', 'Control+G')
  await expect(composer(page)).toHaveAccessibleDescription(/Press Ctrl G for GIFs\./)
  await page.keyboard.press('Control+,')
  await page.getByRole('button', { name: 'Shortcuts' }).click()
  await expect(page.getByRole('dialog', { name: 'Settings' })).toContainText('GIF picker')
})

test('shortcuts do not stack overlays or hijack other surfaces', async ({ page }) => {
  const input = composer(page)
  await input.click()
  // While the GIF picker is open, search, settings and help stay closed.
  await page.keyboard.press('Control+g')
  await expect(gifSearch(page)).toBeFocused()
  for (const key of ['Control+k', 'Control+,', 'Control+/']) await page.keyboard.press(key)
  await expect(page.locator('dialog[open]')).toHaveCount(0)
  await expect(picker(page)).toBeVisible()
  await page.keyboard.press('Escape')
  // While help is open, Ctrl+G and Ctrl+K do nothing.
  await page.keyboard.press('Control+/')
  await expect(help(page)).toBeVisible()
  await page.keyboard.press('Control+g')
  await page.keyboard.press('Control+k')
  await expect(picker(page)).toHaveCount(0)
  await expect(search(page)).toHaveCount(0)
  await expect(help(page)).toBeVisible()
  await page.keyboard.press('Escape')
  // In the emoji picker Ctrl+G switches to GIFs.
  await page.getByRole('button', { name: 'Choose an emoji', exact: true }).click()
  await page.keyboard.press('Control+g')
  await expect(page.getByRole('tab', { name: 'GIFs', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  await expect(gifSearch(page)).toBeFocused()
  await page.keyboard.press('Escape')
  // Unintended extra modifiers are not the shortcut.
  await input.click()
  await page.keyboard.press('Control+Shift+g')
  await page.keyboard.press('Control+Alt+k')
  await expect(picker(page)).toHaveCount(0)
  await expect(search(page)).toHaveCount(0)
})

test('held keys do not toggle overlays repeatedly', async ({ page }) => {
  await composer(page).click()
  await page.keyboard.down('Control')
  await page.keyboard.down('g')
  await page.keyboard.down('g')
  await page.keyboard.down('g')
  await page.keyboard.up('g')
  await page.keyboard.up('Control')
  await expect(picker(page)).toBeVisible()
  await page.keyboard.press('Escape')
  await page.keyboard.down('Control')
  await page.keyboard.down('k')
  await page.keyboard.down('k')
  await page.keyboard.up('k')
  await page.keyboard.up('Control')
  await expect(search(page)).toBeVisible()
})

test('Ctrl+G still opens the picker after Alt-arrow navigation', async ({ page }) => {
  await blur(page)
  await page.keyboard.press('Alt+ArrowDown')
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toHaveCount(0)
  await page.keyboard.press('Alt+ArrowUp')
  await expect(composer(page)).toBeVisible()
  await page.keyboard.press('Control+g')
  await expect(picker(page)).toBeVisible()
  await expect(gifSearch(page)).toBeFocused()
})

test('choosing a search result returns focus to the message box', async ({ page }) => {
  await composer(page).click()
  await page.keyboard.press('Control+k')
  const query = page.getByRole('textbox', { name: 'Search workspace' })
  await expect(query).toBeFocused()
  await query.fill('development')
  await page.keyboard.press('Enter')
  await expect(search(page)).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeFocused()
})

test('a shortcut does not close its overlay from under a confirm prompt', async ({ page }) => {
  await blur(page)
  await page.keyboard.press('Control+,')
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await expect(settings).toBeVisible()
  await settings.getByRole('button', { name: 'Account' }).click()
  await settings.getByRole('button', { name: 'Delete account' }).click()
  const prompt = page.getByRole('dialog', { name: 'Delete your account?' })
  await expect(prompt).toBeVisible()
  for (const key of ['Control+,', 'Control+k', 'Control+/', 'Control+g'])
    await page.keyboard.press(key)
  await expect(prompt).toBeVisible()
  await expect(settings).toBeVisible()
  await expect(page.locator('dialog[open]')).toHaveCount(2)
  // Dismissing the prompt leaves Settings, which the same key then closes.
  await page.keyboard.press('Escape')
  await expect(prompt).toHaveCount(0)
  await page.keyboard.press('Control+,')
  await expect(settings).toHaveCount(0)
})
