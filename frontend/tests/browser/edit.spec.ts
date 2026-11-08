import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('edits your own message in place and keeps its replies', async ({ page }) => {
  const composer = page.getByRole('combobox', { name: 'Message #general' })
  await composer.fill('helo wrold')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  const mine = page.locator('.message-row.is-mine:not(.is-pending)').filter({ hasText: 'helo wrold' })
  await expect(mine).toHaveCount(1)
  // The demo network answers with a reply quoting your message.
  await expect(page.locator('.reply-excerpt').filter({ hasText: 'helo wrold' })).toHaveCount(1)

  // Other people's messages offer no editing.
  // (A just-opened menu can close again while the timeline settles, so retry the click.)
  const theirs = page.locator('.message-row:not(.is-mine)').last()
  await expect(async () => {
    await theirs.click({ button: 'right' })
    await expect(page.getByRole('menuitem', { name: 'Copy text' })).toBeVisible({ timeout: 1000 })
    await expect(page.getByRole('menuitem', { name: 'Edit message' })).toHaveCount(0)
  }).toPass()
  await page.keyboard.press('Escape')

  // Toolbar pencil: Escape cancels without sending anything.
  const edits: string[] = []
  page.on('request', (r) => {
    if (r.method() === 'PATCH' && r.url().includes('/messages/')) edits.push(r.url())
  })
  await mine.hover()
  await mine.getByRole('button', { name: 'Edit message' }).click()
  const editor = page.locator('.message-row.is-editing').getByRole('textbox', { name: 'Edit message' })
  await expect(editor).toBeFocused()
  await expect(editor).toHaveValue('helo wrold')
  await editor.fill('never sent')
  await editor.press('Escape')
  await expect(editor).toHaveCount(0)
  await expect(mine).toContainText('helo wrold')
  expect(edits).toEqual([])

  // Up in the empty composer edits your newest message; Enter saves.
  await composer.focus()
  await composer.press('ArrowUp')
  await expect(editor).toBeFocused()
  await editor.fill('hello **world**')
  await editor.press('Enter')
  await expect(editor).toHaveCount(0)
  const edited = page.locator('.message-row.is-mine').filter({ hasText: 'hello world' })
  await expect(edited).toHaveCount(1)
  await expect(edited.locator('.fmt-bold').filter({ hasText: 'world' })).toHaveCount(1)
  await expect(edited.getByRole('button', { name: /^Edited .*Show earlier versions$/ })).toHaveText(
    '(edited)',
  )
  await expect(page.locator('.message-row.is-mine').filter({ hasText: 'helo wrold' })).toHaveCount(
    0,
  )
  expect(edits).toHaveLength(1)
  // The reply quoting it follows the edit.
  await expect(page.locator('.reply-excerpt').filter({ hasText: 'hello world' })).toHaveCount(1)

  // The marker reveals the earlier version.
  await edited.getByRole('button', { name: /Show earlier versions/ }).click()
  await expect(edited.getByRole('list', { name: 'Earlier versions' })).toContainText('helo wrold')

  // The context menu offers Edit too, and the editor starts from the Markdown you typed.
  await expect(async () => {
    await edited.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Edit message' }).click({ timeout: 1000 })
  }).toPass()
  await expect(editor).toHaveValue('hello **world**')
  await editor.press('Escape')

  // Edits survive a reload.
  await page.reload()
  await expect(
    page.locator('.message-row.is-mine').filter({ hasText: 'hello world' }).getByText('(edited)'),
  ).toBeVisible()
})

test('hides editing where the network cannot carry it', async ({ page }) => {
  await page.getByRole('button', { name: 'OFTC', exact: true }).click()
  await page.locator('.conversation-link').filter({ hasText: 'community' }).click()
  const composer = page.getByRole('combobox', { name: 'Message #community' })
  await expect(composer).toBeEnabled()
  await composer.fill('cannot edit here')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  const mine = page
    .locator('.message-row.is-mine:not(.is-pending)')
    .filter({ hasText: 'cannot edit here' })
  await expect(mine).toHaveCount(1)
  await expect(async () => {
    await mine.click({ button: 'right' })
    await expect(page.getByRole('menuitem', { name: 'Copy text', exact: true })).toBeVisible({
      timeout: 1000,
    })
    await expect(page.getByRole('menuitem', { name: 'Edit message' })).toHaveCount(0)
  }).toPass()
  await page.keyboard.press('Escape')
  await mine.hover()
  await expect(mine.getByRole('button', { name: 'Edit message' })).toHaveCount(0)
  await composer.focus()
  await composer.press('ArrowUp')
  await expect(page.getByRole('textbox', { name: 'Edit message' })).toHaveCount(0)
})

test('does not offer editing without an account login on the network', async ({ page }) => {
  await page.route('**/api/v1/bootstrap', async (route) => {
    const state = await (await route.fetch()).json()
    for (const n of state.networks) n.account = ''
    await route.fulfill({ json: state })
  })
  await page.reload()
  const composer = page.getByRole('combobox', { name: 'Message #general' })
  await expect(composer).toBeEnabled()
  await composer.fill('no account to edit from')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  const mine = page
    .locator('.message-row.is-mine:not(.is-pending)')
    .filter({ hasText: 'no account to edit from' })
  await expect(mine).toHaveCount(1)
  const item = page.getByRole('menuitem', { name: /Edit message/ })
  await expect(async () => {
    await mine.click({ button: 'right' })
    await expect(item).toHaveAttribute('aria-disabled', 'true', { timeout: 1000 })
    await expect(item).toContainText('needs account', { timeout: 1000 })
  }).toPass()
  await page.keyboard.press('Escape')
  await mine.hover()
  const button = mine.getByRole('button', { name: 'Edit message' })
  await expect(button).toBeDisabled()
  await expect(button).toHaveAttribute('title', 'Editing needs a logged-in account on this network')
})
