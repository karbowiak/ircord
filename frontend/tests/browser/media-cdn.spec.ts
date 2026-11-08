import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'
import type { Message } from '../../shared/types/chat'

const original = 'https://untrusted-images.example/dance.gif'
const copy = `https://cdn.example/media/${'a'.repeat(64)}`
const gif = Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64')

async function storedMessage(page: Page, extra: Partial<Message> = {}) {
  let current: Message
  let sequence = 0
  let write: ((value: string) => void) | undefined
  const originals: string[] = []
  await page.route('https://untrusted-images.example/**', (route) => {
    originals.push(route.request().url())
    return route.abort()
  })
  await page.route('https://cdn.example/**', (route) =>
    route.fulfill({ contentType: 'image/gif', body: gif }),
  )
  await page.route('**/api/v1/bootstrap', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    body.capabilities.mediaCDN = true
    sequence = Math.max(sequence, body.cursor)
    await route.fulfill({ response, json: body })
  })
  await page.route('**/api/v1/conversations/general/messages*', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    current ??= {
      ...body.items.at(-1),
      text: original,
      media: [{ source: original, status: 'loading', kind: 'image' }],
      ...extra,
    }
    body.items[body.items.length - 1] = current
    await route.fulfill({ response, json: body })
  })
  await page.routeWebSocket('**/api/v1/events*', (socket) => {
    write = (value) => socket.send(value)
    const server = socket.connectToServer()
    server.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      sequence = Math.max(sequence, event.seq ?? 0)
      socket.send(String(raw))
    })
  })
  await signIn(page)
  return {
    originals,
    async update(media: NonNullable<Message['media']>) {
      current = { ...current!, media }
      write!(JSON.stringify({ v: 1, seq: ++sequence, type: 'message.updated', data: current }))
    },
    row: page
      .locator('.message-row')
      .filter({ has: page.locator(`a[href="${original}"]`) })
      .first(),
  }
}

test('stored GIFs show loading, follow live readiness and never fetch the original host', async ({
  page,
}) => {
  const state = await storedMessage(page)
  await expect(page.getByText('Preparing media…', { exact: true })).toBeVisible()
  await expect(page.locator(`img[src="${original}"]`)).toHaveCount(0)
  await page.screenshot({ path: 'test-results/media-cdn-loading-desktop.png' })
  await state.update([
    {
      source: original,
      status: 'ready',
      kind: 'image',
      url: copy,
      hash: 'a'.repeat(64),
      animated: true,
      contentType: 'image/gif',
      width: 1,
      height: 1,
      size: gif.length,
    },
  ])
  const image = page.getByRole('button', { name: 'View GIF from untrusted-images.example' })
  await expect(image).toBeVisible()
  await expect(image.locator('img')).toHaveAttribute('src', copy)
  await expect(image.locator('canvas.media-still')).toBeVisible()
  await image.hover()
  await expect(image.locator('canvas.media-still')).toBeHidden()
  await page.mouse.move(0, 0)
  await expect(image.locator('canvas.media-still')).toBeVisible()
  expect(state.originals).toEqual([])
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(image).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  await page.screenshot({ path: 'test-results/media-cdn-ready-mobile.png' })
  await image.click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByRole('dialog').locator('img')).toHaveAttribute('src', copy)
  expect(state.originals).toEqual([])
})

test('failed and removed media keep the original link without fetching it', async ({ page }) => {
  const state = await storedMessage(page)
  await expect(page.getByText('Preparing media…', { exact: true })).toBeVisible()
  await state.update([
    { source: original, status: 'failed', kind: 'image', reason: 'Media could not be stored.' },
  ])
  await expect(page.getByText('Media could not be stored.', { exact: true })).toBeVisible()
  await expect(page.locator(`a[href="${original}"]`).first()).toBeVisible()
  await state.update([
    { source: original, status: 'removed', kind: 'image', reason: 'This media was removed.' },
  ])
  await expect(page.getByText('This media was removed.', { exact: true })).toBeVisible()
  expect(state.originals).toEqual([])
  await page.screenshot({ path: 'test-results/media-cdn-removed.png' })
})

test('server preview thumbnails are also held until their stored copy is ready', async ({
  page,
}) => {
  const article = 'https://article.example/post'
  const state = await storedMessage(page, {
    text: article,
    previewSource: 'server',
    preview: { url: article, title: 'A shared article', image: original, kind: 'article' },
  })
  await expect(page.getByText('A shared article', { exact: true })).toBeVisible()
  await expect(page.getByText('Preparing media…', { exact: true })).toBeVisible()
  expect(state.originals).toEqual([])
  await state.update([
    {
      source: original,
      status: 'ready',
      kind: 'image',
      url: copy,
      animated: true,
      contentType: 'image/gif',
    },
  ])
  await expect(page.locator('.link-card img')).toHaveAttribute('src', copy)
  expect(state.originals).toEqual([])
})
