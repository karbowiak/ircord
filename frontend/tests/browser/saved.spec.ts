import { test, expect, type Locator } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const coffee = (page: import('@playwright/test').Page) =>
  page.locator('.message-row:not(.is-mine)').filter({ hasText: 'coffee' }).first()
const saveButton = (row: Locator) =>
  row.getByRole('button', {
    name: /^(Save for later|Remove from saved|Saving…|Removing from saved…)$/,
  })

test('save from the toolbar, find it in Saved messages and jump back to it', async ({
  page,
  context,
}) => {
  // Another device of the same account is open before saving, and follows along live.
  const other = await context.newPage()
  await other.goto('/')
  await expect(other.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  const otherRow = coffee(other)
  await otherRow.hover()
  await expect(saveButton(otherRow)).toHaveAccessibleName('Save for later')

  const row = coffee(page)
  const text = (await row.locator('.message-text').innerText()).trim()
  await row.hover()
  await row.getByRole('button', { name: 'Save for later' }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Saved for later.' })).toBeVisible()
  await row.hover()
  await expect(saveButton(row)).toHaveAttribute('aria-pressed', 'true')
  await expect(saveButton(row)).toHaveAccessibleName('Remove from saved')
  await otherRow.hover()
  await expect(saveButton(otherRow)).toHaveAccessibleName('Remove from saved')

  // A half-written message elsewhere survives opening the saved one.
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  const draft = page.getByRole('combobox', { name: 'Message #development' })
  await draft.fill('half-written thought')
  await page.getByRole('button', { name: 'Saved messages' }).click()
  const dialog = page.getByRole('dialog', { name: 'Saved messages' })
  const item = dialog.locator('.saved-item').filter({ hasText: text })
  await expect(item).toContainText('#general')
  // Keyboard: the filter field drives the list.
  await dialog.getByRole('textbox', { name: 'Filter saved messages' }).press('Enter')
  await expect(dialog).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeVisible()
  await expect(page.locator('.message-row.is-highlighted')).toContainText(text)
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await expect(draft).toHaveValue('half-written thought')

  // Saved state is the account's: it survives a reload, and a filter narrows the list.
  await page.reload()
  await page.getByRole('button', { name: 'Saved messages' }).click()
  await dialog.getByRole('textbox', { name: 'Filter saved messages' }).fill('no such saved text')
  await expect(dialog).toContainText('No saved messages match')
  await dialog.getByRole('textbox', { name: 'Filter saved messages' }).fill('')
  await expect(item).toBeVisible()

  // Removing it drops only the bookmark, here and on the other device.
  await item.getByRole('button', { name: /^Remove from saved/ }).click()
  await expect(dialog.locator('.saved-item')).toHaveCount(0)
  await expect(dialog).toContainText('Nothing saved yet')
  await otherRow.hover()
  await expect(saveButton(otherRow)).toHaveAccessibleName('Save for later')
  await other.close()
})

test('the message menu saves and removes too, for keyboard and touch users', async ({ page }) => {
  const row = coffee(page)
  await row.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Save for later' }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Saved for later.' })).toBeVisible()
  await row.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Remove from saved' }).click()
  await expect(
    page.getByRole('status').filter({ hasText: 'Removed from saved messages.' }),
  ).toBeVisible()
  await row.hover()
  await expect(saveButton(row)).toHaveAccessibleName('Save for later')
})

test('a save in flight or failed is never shown as saved', async ({ page }) => {
  let answer: () => void = () => {}
  const held = new Promise<void>((resolve) => (answer = resolve))
  await page.route('**/api/v1/saved-messages/**', async (route) => {
    if (route.request().method() !== 'PUT') return route.continue()
    await held
    await route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ statusMessage: 'Storage is unavailable' }),
    })
  })
  const row = coffee(page)
  await row.hover()
  await row.getByRole('button', { name: 'Save for later' }).click()
  // While the server has not answered, the button says so and is not pressed.
  await row.hover()
  await expect(saveButton(row)).toHaveAccessibleName('Saving…')
  await expect(saveButton(row)).toHaveAttribute('aria-pressed', 'false')
  await expect(saveButton(row)).toHaveAttribute('aria-busy', 'true')
  answer()
  await expect(
    page.getByRole('status').filter({ hasText: 'Couldn’t save the message' }),
  ).toBeVisible()
  await row.hover()
  await expect(saveButton(row)).toHaveAccessibleName('Save for later')
  await expect(saveButton(row)).toHaveAttribute('aria-pressed', 'false')
  await page.getByRole('button', { name: 'Saved messages' }).click()
  await expect(page.getByRole('dialog', { name: 'Saved messages' })).toContainText(
    'Nothing saved yet',
  )
})

test('a saved message far beyond the loaded timeline opens in place', async ({ page }) => {
  const load = await page.request.post('/api/v1/_demo/busy', {
    data: { members: 20, history: 600, rate: 0, churn: 0, seconds: 5 },
  })
  expect(load.ok()).toBe(true)
  const link = page.locator('.conversation-link').filter({ hasText: /^busy/ })
  await link.click()
  await expect(page.getByRole('combobox', { name: 'Message #busy' })).toBeVisible()
  // Find an early message of the channel through the API, far past the newest page.
  const bootstrap = await (await page.request.get('/api/v1/bootstrap')).json()
  const busy = bootstrap.conversations.find((c: { name: string }) => c.name === 'busy')
  let page100 = await (
    await page.request.get(`/api/v1/conversations/${busy.id}/messages?limit=100`)
  ).json()
  for (let i = 0; i < 4 && page100.nextCursor; i++)
    page100 = await (
      await page.request.get(
        `/api/v1/conversations/${busy.id}/messages?limit=100&before=${encodeURIComponent(page100.nextCursor)}`,
      )
    ).json()
  const old = page100.items.find((m: { kind: string }) => m.kind === 'message')
  expect(old).toBeTruthy()
  const saved = await page.request.put(`/api/v1/saved-messages/${encodeURIComponent(old.id)}`, {
    data: { conversationId: busy.id },
  })
  expect(saved.ok()).toBe(true)

  await page.getByRole('button', { name: 'Saved messages' }).click()
  const dialog = page.getByRole('dialog', { name: 'Saved messages' })
  await dialog.locator('.saved-item').first().locator('.saved-open').click()
  await expect(dialog).toHaveCount(0)
  await expect(page.locator('.message-row.is-highlighted')).toContainText(old.text)
  // The timeline is a window into the past until you return to the present.
  const present = page.getByRole('button', { name: /Viewing older messages · Jump to present/ })
  await expect(present).toBeVisible()
  await present.click()
  await expect(page.getByRole('button', { name: /Viewing older messages/ })).toHaveCount(0)
})
