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
  // The channel picker and the host and date filters fit too, and can be used by touch.
  await dialog.getByRole('button', { name: 'Choose channels: #general' }).click()
  await expect(dialog.getByRole('checkbox', { name: '#development' })).toBeVisible()
  await expect(dialog.getByRole('textbox', { name: 'Host' })).toBeVisible()
  await expect(dialog.getByLabel('Posted until')).toBeVisible()
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth),
  ).toBeLessThanOrEqual(0)
  const box = await dialog.getByRole('textbox', { name: 'Host' }).boundingBox()
  expect(box!.x + box!.width).toBeLessThanOrEqual(390)
})

test('links can be narrowed to chosen channels, a host and the days they were posted', async ({
  page,
}) => {
  await send(page, 'general', 'read https://news.example/today')
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await send(page, 'development', 'docs at https://docs.example/guide')
  await page.locator('.conversation-link').filter({ hasText: 'general' }).click()

  await page.getByRole('button', { name: 'Links in #general' }).click()
  const dialog = page.getByRole('dialog', { name: 'Links' })
  const cards = dialog.locator('.catalog-card')
  const byHost = (host: string) => cards.filter({ hasText: host })
  await expect(byHost('news.example')).toHaveCount(1)
  await expect(byHost('docs.example')).toHaveCount(0)

  // Several channels at once: #general and #development, then #development alone.
  const picker = dialog.getByRole('button', { name: 'Choose channels: #general' })
  await picker.click()
  const channels = dialog.getByRole('group', { name: 'Channels to list' })
  // Opening moves focus into the picker, so its boxes come next for the keyboard.
  await expect(channels.getByRole('checkbox').first()).toBeFocused()
  await channels.getByRole('checkbox', { name: '#development' }).check()
  await expect(dialog.getByRole('button', { name: 'Choose channels: 2 channels' })).toBeVisible()
  await expect(byHost('docs.example')).toHaveCount(1)
  await expect(byHost('news.example')).toHaveCount(1)
  await expect(byHost('musicforprogramming.net')).toHaveCount(0)
  // Neither scope choice applies to a custom selection.
  await expect(dialog.getByRole('radio', { checked: true })).toHaveCount(0)
  await channels.getByRole('checkbox', { name: '#general' }).uncheck()
  await expect(byHost('news.example')).toHaveCount(0)
  await expect(byHost('docs.example')).toHaveCount(1)
  // Escape closes only the picker, keeping the choice, and focus returns to its button.
  await channels.getByRole('checkbox', { name: '#development' }).press('Escape')
  await expect(channels).toHaveCount(0)
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Choose channels: #development' })).toBeFocused()

  // The scope toggle still switches back to every channel.
  await dialog.getByRole('radio', { name: 'All channels' }).click()
  await expect(byHost('musicforprogramming.net')).toHaveCount(1)

  // Host: a host name, or a pasted address, keeps that host's links; a bad one says why.
  const host = dialog.getByRole('textbox', { name: 'Host' })
  await host.fill('DOCS.example')
  await expect(cards).toHaveCount(1)
  await expect(cards).toContainText('docs.example')
  await host.fill('https://news.example/some/page')
  await expect(cards).toHaveCount(1)
  await expect(cards).toContainText('news.example/today')
  await host.fill('news.example/today')
  await expect(dialog.getByRole('alert')).toContainText('Enter a host such as example.com.')
  await expect(host).toHaveAttribute('aria-invalid', 'true')
  // Enter while the list is replaced by the hint jumps nowhere and keeps the view open.
  await host.press('Enter')
  await expect(dialog).toBeVisible()
  await host.fill('foo_bar.example')
  await expect(dialog.getByRole('alert')).toContainText('Enter a host such as example.com.')
  await host.fill('')
  await expect(byHost('docs.example')).toHaveCount(1)

  // Dates: whole days, both ends included; a reversed range says why.
  const today = await page.evaluate(() => {
    const d = new Date()
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  })
  const fromDate = dialog.getByLabel('Posted from')
  const toDate = dialog.getByLabel('Posted until')
  await fromDate.fill(today)
  await toDate.fill(today)
  await expect(byHost('docs.example')).toHaveCount(1)
  await expect(byHost('news.example')).toHaveCount(1)
  await fromDate.fill('2001-01-01')
  await toDate.fill('2000-01-01')
  await expect(dialog.getByRole('alert')).toContainText('The end date is before the start date.')
  await fromDate.fill('')
  await expect(dialog).toContainText('No links match these filters.')
  await dialog.getByRole('button', { name: 'Clear filters' }).click()
  await expect(toDate).toHaveValue('')
  await expect(byHost('docs.example')).toHaveCount(1)
})
