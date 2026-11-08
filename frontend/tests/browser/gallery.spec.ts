import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

// A 1×1 PNG, served for every remote image so nothing leaves the test machine.
const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=',
  'base64',
)
async function serveRemote(page: Page) {
  const hosts: string[] = []
  await page.route(
    (url) => url.protocol === 'https:' && url.hostname !== '127.0.0.1',
    (route) => {
      hosts.push(new URL(route.request().url()).hostname)
      return route.fulfill({ contentType: 'image/png', body: png })
    },
  )
  return hosts
}
async function send(page: Page, text: string) {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill(text)
  await input.press('Enter')
}

test('the channel gallery lists posted images, opens the viewer and jumps to the post', async ({
  page,
}) => {
  await signIn(page)
  const hosts = await serveRemote(page)
  await send(
    page,
    'gallery one https://i.imgur.com/gallery-one.png https://untrusted.example/two.png',
  )
  // The demo echo bot repeats the message, so the channel has two posts with two images each.
  await expect(page.locator('.message-row').filter({ hasText: 'gallery one' })).toHaveCount(2)

  await page.getByRole('button', { name: 'Gallery' }).click()
  const gallery = page.getByRole('dialog', { name: 'Gallery · #general' })
  await expect(gallery).toBeVisible()
  await expect(gallery.getByText(/^\d+ images · Libera Chat/)).toBeVisible()
  const posted = gallery.getByRole('listitem').filter({
    has: page.getByRole('button', { name: /^Jump to the message: gallery one/ }),
  })
  await expect(posted).toHaveCount(4)
  await expect(posted.getByRole('button', { name: /^View Image from/ })).toHaveCount(2)
  // Under the default “trusted” setting an untrusted host stays a link and is never requested.
  await expect(posted.getByRole('link', { name: /untrusted\.example/ })).toHaveCount(2)
  expect(hosts).not.toContain('untrusted.example')

  await posted
    .getByRole('button', { name: /^View Image from/ })
    .first()
    .click()
  const viewer = page.locator('dialog.image-viewer')
  await expect(viewer).toBeVisible()
  await expect(viewer.getByText(/^1 of \d+$/)).toBeVisible()
  await page.keyboard.press('ArrowRight')
  await expect(viewer.getByText(/^2 of \d+$/)).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(viewer).toBeHidden()
  await expect(gallery).toBeVisible()

  await gallery
    .getByRole('button', { name: /^Jump to the message: gallery one/ })
    .first()
    .click()
  await expect(gallery).toBeHidden()
  await expect(page.locator('.message-row.is-highlighted')).toContainText('gallery one')
})

test('an empty gallery says what it will show', async ({ page }) => {
  await signIn(page)
  await page
    .locator('.conversation-sidebar .conversation-link')
    .filter({ hasText: 'design' })
    .click()
  await page.getByRole('button', { name: 'Gallery' }).click()
  const gallery = page.getByRole('dialog', { name: /^Gallery · #/ })
  await expect(gallery.getByText(/No images here yet/)).toBeVisible()
  await expect(gallery.getByText(/^0 images/)).toBeVisible()
})
