import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const quick = (row: import('@playwright/test').Locator) =>
  row.locator('.message-actions .quick-reaction')

test('quick reactions follow your most recent emoji', async ({ page }) => {
  const row = page.locator('.message-row:not(.is-mine)').filter({ hasText: 'coffee' }).first()
  await row.hover()
  // No history yet: sensible defaults, then the usual actions and a way to the rest.
  await expect(quick(row)).toHaveText(['👍', '❤️', '😂'])
  for (const name of ['Add reaction', 'Reply to message', 'Copy message', 'More actions'])
    await expect(row.getByRole('button', { name, exact: true })).toBeVisible()

  // Picking from the full picker puts that emoji first.
  await row.getByRole('button', { name: 'Add reaction' }).click()
  await page.getByRole('textbox', { name: 'Search emoji' }).fill('party popper')
  await page.getByRole('button', { name: 'Insert party popper' }).click()
  await row.hover()
  await expect(quick(row)).toHaveText(['🎉', '👍', '❤️'])

  // A quick reaction counts as a use too, so it moves to the front.
  await quick(row).nth(2).click()
  await row.hover()
  await expect(quick(row)).toHaveText(['❤️', '🎉', '👍'])
  // The long-press/right-click menu offers the same recent emoji.
  await row.click({ button: 'right' })
  await expect(page.getByRole('menuitem', { name: 'React with ❤️' })).toBeVisible()
  await page.keyboard.press('Escape')

  // The history is the account's: it survives a reload.
  await page.reload()
  await row.hover()
  await expect(quick(row)).toHaveText(['❤️', '🎉', '👍'])
})

test('holding Shift swaps in the alternate actions and never sticks', async ({ page }) => {
  const composer = page.getByRole('combobox', { name: 'Message #general' })
  await composer.fill('my shift test')
  await composer.press('Enter')
  const mine = page.locator('.message-row.is-mine:not(.is-pending)').filter({ hasText: 'my shift test' })
  await expect(mine).toHaveCount(1)
  const unread = mine.getByRole('button', { name: 'Mark as unread' })
  const reply = mine.getByRole('button', { name: 'Reply to message' })

  // Shift pressed while already hovering, released without moving the pointer.
  await mine.hover()
  await expect(reply).toBeVisible()
  await page.keyboard.down('Shift')
  await expect(unread).toBeVisible()
  await expect(mine.getByRole('button', { name: 'Delete message' })).toBeVisible()
  await expect(reply).toHaveCount(0)
  await page.keyboard.up('Shift')
  await expect(reply).toBeVisible()
  await expect(unread).toHaveCount(0)

  // Shift pressed before hovering.
  await page.mouse.move(5, 5)
  await page.keyboard.down('Shift')
  await mine.hover()
  await expect(unread).toBeVisible()
  // Losing focus while held resets it.
  await page.evaluate(() => window.dispatchEvent(new Event('blur')))
  await expect(reply).toBeVisible()
  await page.keyboard.up('Shift')

  // Shift alone does nothing: no dialog, no unread state.
  await page.keyboard.press('Shift')
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.locator('.unread-divider')).toHaveCount(0)

  // Other people's messages offer no Delete in Shift mode.
  const theirs = page.locator('.message-row:not(.is-mine)').filter({ hasText: 'coffee' }).first()
  await theirs.hover()
  await page.keyboard.down('Shift')
  await expect(theirs.getByRole('button', { name: 'Mark as unread' })).toBeVisible()
  await expect(theirs.getByRole('button', { name: 'Delete message' })).toHaveCount(0)
  await page.keyboard.up('Shift')

  // Keyboard users: Shift+Tab inside the toolbar keeps its buttons in place.
  await theirs.hover()
  await theirs.getByRole('button', { name: 'Reply to message' }).focus()
  await page.keyboard.press('Tab')
  await expect(theirs.getByRole('button', { name: 'Copy message' })).toBeFocused()
  await page.keyboard.press('Shift+Tab')
  await expect(theirs.getByRole('button', { name: 'Reply to message' })).toBeFocused()

  // A button clicked with the mouse keeps focus, yet Shift still swaps the hovered toolbar.
  await theirs.getByRole('button', { name: 'Copy message' }).click()
  await page.keyboard.down('Shift')
  await expect(theirs.getByRole('button', { name: 'Mark as unread' })).toBeVisible()
  await page.keyboard.up('Shift')
})

test('mark as unread survives auto-read and reload until you read it again', async ({ page }) => {
  const general = page.locator('.conversation-link').filter({ hasText: /^general$/ })
  const rows = page.locator('.message-row:not(.is-mine)')
  const target = rows.nth((await rows.count()) - 2)
  const text = (await target.locator('.message-text').innerText()).trim()

  await target.hover()
  await page.keyboard.down('Shift')
  await target.getByRole('button', { name: 'Mark as unread' }).click()
  await page.keyboard.up('Shift')
  await expect(general).toHaveClass(/has-unread/)
  const divider = page.locator('.virtual-row').filter({ has: page.locator('.unread-divider') })
  await expect(divider).toContainText(text)

  // Focus returning and new messages don't immediately read it again.
  await page.evaluate(() => window.dispatchEvent(new Event('focus')))
  await page.waitForTimeout(600)
  await expect(general).toHaveClass(/has-unread/)

  // It is saved on the account: still unread, at the same message, after a reload.
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await page.waitForTimeout(600)
  await expect(general).toHaveClass(/has-unread/)
  await expect(divider).toContainText(text)

  // Leaving and coming back reads it.
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await general.click()
  await expect(general).not.toHaveClass(/has-unread/)

  // The context menu offers the same action for keyboard and touch users.
  await target.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Mark as unread' }).click()
  await expect(general).toHaveClass(/has-unread/)
  await general.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Mark as read' }).click()
  await expect(general).not.toHaveClass(/has-unread/)
})
