import { test, expect, type Page } from '@playwright/test'
import { chooseOption, signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

/** Sends a message and waits for the demo's echo bot to post it back, so counts are settled. */
async function send(page: Page, channel: string, text: string) {
  const input = page.getByRole('combobox', { name: `Message #${channel}` })
  const rows = page
    .locator('.message-row')
    .filter({ hasText: text.replaceAll('`', '').slice(0, 24) })
  const before = await rows.count()
  await input.fill(text)
  await input.press('Enter')
  await expect(rows).toHaveCount(before + 2)
}

test('links in a channel are grouped, filtered, copied and opened at their post', async ({
  page,
  context,
}) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await send(
    page,
    'general',
    'two reads: https://news.example/article?id=1 and https://news.example/article?id=2',
  )
  await send(page, 'general', 'worth it again https://news.example/article?id=1.')
  await send(page, 'general', '`https://news.example/in-code`')

  // A draft in another channel survives jumping to a link's post.
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  const draft = page.getByRole('combobox', { name: 'Message #development' })
  await draft.fill('half-written thought')
  await page.locator('.conversation-link').filter({ hasText: 'general' }).click()

  await page.getByRole('button', { name: 'Links in #general' }).click()
  const dialog = page.getByRole('dialog', { name: 'Links' })
  const cards = dialog.locator('.catalog-card')
  await expect(cards).toHaveCount(2)
  // The same article posted twice is one card, a different query string is another resource,
  // and a link written as code is not a link.
  const byTitle = (title: RegExp) =>
    cards.filter({ has: page.locator('.catalog-card-title', { hasText: title }) })
  const first = byTitle(/article\?id=1$/)
  await expect(first).toContainText('Posted 4 times')
  await expect(first).toContainText('worth it again')
  await expect(byTitle(/article\?id=2$/)).toHaveCount(1)
  await expect(
    first.getByRole('link', { name: 'Open link: https://news.example/article?id=1' }),
  ).toHaveAttribute('href', 'https://news.example/article?id=1')
  await first.getByRole('button', { name: 'Posted 4 times · show posts' }).click()
  await expect(first.locator('.catalog-card-sources li')).toHaveCount(4)

  await first.getByRole('button', { name: /^Copy link/ }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Link copied.' })).toBeVisible()
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
    'https://news.example/article?id=1',
  )

  // Filters narrow the list; an empty result says so.
  const filter = dialog.getByRole('textbox', { name: 'Filter links' })
  // Message text matches too, and counts follow the filter: two posts say “worth it”.
  await filter.fill('worth it')
  await expect(cards).toHaveCount(1)
  await expect(cards).toContainText('Posted 2 times')
  await filter.fill('')
  await chooseOption(page, dialog.getByRole('combobox', { name: 'Link type' }), 'Images')
  await expect(dialog).toContainText('No links match these filters.')
  await chooseOption(page, dialog.getByRole('combobox', { name: 'Link type' }), 'All types')
  await expect(cards).toHaveCount(2)

  // Every channel: the demo's #music link appears with its channel.
  await dialog.getByRole('radio', { name: 'All channels' }).click()
  await expect(cards.filter({ hasText: 'musicforprogramming.net' })).toContainText('#music')

  // Keyboard: Enter jumps to the newest post of the chosen card.
  await filter.fill('worth it')
  await expect(cards).toHaveCount(1)
  await filter.press('Enter')
  await expect(dialog).toHaveCount(0)
  await expect(page.locator('.message-row.is-highlighted')).toContainText('worth it again')
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await expect(draft).toHaveValue('half-written thought')
})

test('the links view fits a phone screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await send(
    page,
    'general',
    'see https://news.example/a-rather-long-article-title-that-should-wrap?with=query&and=more',
  )
  await page.getByRole('button', { name: 'Links in #general' }).click()
  const dialog = page.getByRole('dialog', { name: 'Links' })
  await expect(dialog.locator('.catalog-card')).toHaveCount(1)
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - window.innerWidth,
  )
  expect(overflow).toBeLessThanOrEqual(0)
  await expect(dialog.getByRole('button', { name: /^Jump to the message/ })).toBeVisible()
})
