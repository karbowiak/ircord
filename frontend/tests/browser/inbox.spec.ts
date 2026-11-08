import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

const inboxButton = (page: Page) => page.getByRole('button', { name: /^Inbox(, \d+ unread)?$/ })
const sidebarLink = (page: Page, name: string) =>
  page.locator('.conversation-link').filter({ hasText: name })

test('mentions, replies and direct messages land in one inbox that stays separate from read state', async ({
  page,
  context,
}) => {
  await signIn(page)
  // The demo seeds an unread mention in #development and an unread message from Jules.
  await expect(inboxButton(page)).toHaveAccessibleName('Inbox, 2 unread')
  const other = await context.newPage()
  await other.goto('/')
  await expect(inboxButton(other)).toHaveAccessibleName('Inbox, 2 unread')

  await inboxButton(page).click()
  const dialog = page.getByRole('dialog', { name: 'Inbox' })
  const mention = dialog.locator('.inbox-item').filter({ hasText: 'would love to hear' })
  const jules = dialog.locator('.inbox-item').filter({ hasText: 'Good to see you here' })
  await expect(mention).toContainText('#development')
  await expect(mention).toContainText('Mention')
  await expect(jules).toContainText('@jules')
  await expect(jules).toContainText('Direct message')

  // Focused views; the keyboard moves between them.
  await dialog.getByRole('tab', { name: 'Mentions' }).click()
  await expect(dialog.locator('.inbox-item')).toHaveCount(1)
  await page.keyboard.press('ArrowRight')
  await expect(dialog.getByRole('tab', { name: 'Replies' })).toHaveAttribute('aria-selected', 'true')
  await expect(dialog).toContainText('No replies')
  await page.keyboard.press('ArrowRight')
  await expect(dialog.locator('.inbox-item')).toHaveCount(2)
  await dialog.getByRole('tab', { name: 'All' }).click()

  // Dismissing hides the item on every device but leaves the conversation unread.
  await jules.getByRole('button', { name: 'Dismiss from inbox' }).click()
  await expect(jules).toHaveCount(0)
  await expect(inboxButton(page)).toHaveAccessibleName('Inbox, 1 unread')
  await expect(inboxButton(other)).toHaveAccessibleName('Inbox, 1 unread')
  await expect(sidebarLink(page, 'jules').locator('.unread-dot, .mention-badge')).toHaveCount(1)
  await dialog.getByRole('tab', { name: 'Dismissed' }).click()
  await dialog.locator('.inbox-item').filter({ hasText: 'Good to see you here' }).getByRole('button', { name: 'Restore to inbox' }).click()
  await expect(dialog).toContainText('Nothing dismissed')
  await expect(inboxButton(page)).toHaveAccessibleName('Inbox, 2 unread')
  // Settled on the other device too, so the read below is what changes its count.
  await expect(inboxButton(other)).toHaveAccessibleName('Inbox, 2 unread')
  await dialog.getByRole('tab', { name: 'All' }).click()

  // Opening an item jumps to it and reads that conversation the usual way; the item stays.
  await mention.locator('.saved-open').click()
  await expect(dialog).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeVisible()
  await expect(page.locator('.message-row.is-highlighted')).toContainText('would love to hear')
  await expect(inboxButton(page)).toHaveAccessibleName('Inbox, 1 unread')
  await expect(inboxButton(other)).toHaveAccessibleName('Inbox, 1 unread')
  await inboxButton(page).click()
  await expect(mention).toBeVisible()
  await expect(mention).not.toHaveClass(/is-unread/)
  await dialog.getByRole('tab', { name: 'Unread' }).click()
  await expect(dialog.locator('.inbox-item')).toHaveCount(1)
  await expect(dialog.locator('.inbox-item')).toContainText('Good to see you here')
  await page.keyboard.press('Escape')

  // A reply to your own message arrives live (the demo's echo bot replies to what you send).
  await sidebarLink(page, 'general').click()
  await page.getByRole('combobox', { name: 'Message #general' }).fill('inbox reply check')
  await page.keyboard.press('Enter')
  await expect(page.locator('.message-row').filter({ hasText: 'inbox reply check' })).toHaveCount(2)
  await inboxButton(page).click()
  await dialog.getByRole('tab', { name: 'Replies' }).click()
  await expect(dialog.locator('.inbox-item')).toHaveCount(1)
  await expect(dialog.locator('.inbox-item')).toContainText('inbox reply check')
  await expect(dialog.locator('.inbox-item')).toContainText('#general')
  // Keyboard: choose and open.
  await page.keyboard.press('Enter')
  await expect(dialog).toHaveCount(0)
  await expect(page.locator('.message-row.is-highlighted')).toContainText('inbox reply check')

  // The inbox is the account's: it survives a reload.
  await page.reload()
  await inboxButton(page).click()
  await expect(dialog.locator('.inbox-item')).toHaveCount(4)
  await other.close()
})

test.describe('on a phone', () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true })
  test('the inbox fits the screen', async ({ page }) => {
    await signIn(page)
    await inboxButton(page).tap()
    const dialog = page.getByRole('dialog', { name: 'Inbox' })
    await expect(dialog.locator('.inbox-item').first()).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
    await dialog.getByRole('tab', { name: 'Dismissed' }).tap()
    await expect(dialog).toContainText('Nothing dismissed')
  })
})
