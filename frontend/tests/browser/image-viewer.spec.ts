import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

/** Uploads a large generated PNG and opens it from the timeline. */
async function openLargeImage(page: Page) {
  const maker = await page.context().newPage()
  await maker.setViewportSize({ width: 1200, height: 750 })
  await maker.setContent('<body style="margin:0;height:100vh;background:#5b8def">')
  const buffer = await maker.screenshot({ scale: 'device' })
  await maker.close()
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await page
    .locator('input[type=file][aria-hidden=true]')
    .first()
    .setInputFiles({ name: 'wide.png', mimeType: 'image/png', buffer })
  await input.click()
  await input.press('Enter')
  const thumb = page.getByRole('button', { name: 'View wide.png' }).last()
  await thumb.click()
  await expect(page.getByRole('dialog', { name: 'wide.png' })).toBeVisible()
  await expect(page.locator('.image-viewer-stage img')).toBeVisible()
  return thumb
}

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('the image viewer fits the image in the window and closes three ways', async ({ page }) => {
  const thumb = await openLargeImage(page)
  const viewport = page.viewportSize()!
  const box = (await page.locator('.image-viewer-stage img').boundingBox())!
  expect(box.x).toBeGreaterThanOrEqual(0)
  expect(box.y + box.height).toBeLessThanOrEqual(viewport.height)
  expect(box.x + box.width).toBeLessThanOrEqual(viewport.width)
  // Centred horizontally.
  expect(Math.abs(box.x + box.width / 2 - viewport.width / 2)).toBeLessThan(2)
  await expect(page.getByRole('link', { name: 'Open original' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Download' })).toHaveAttribute('download', 'wide.png')
  const violations = await new AxeBuilder({ page })
    .include('.image-viewer')
    .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
    .analyze()
  expect(violations.violations.map((v) => v.id)).toEqual([])

  // Esc closes and returns focus to the thumbnail.
  await page.keyboard.press('Escape')
  await expect(page.locator('.image-viewer')).toHaveCount(0)
  await expect(thumb).toBeFocused()

  // Clicking the image keeps it open; clicking beside it closes.
  await thumb.click()
  await page.locator('.image-viewer-stage img').click()
  await expect(page.locator('.image-viewer')).toBeVisible()
  await page.mouse.click(viewport.width / 2, viewport.height - 4)
  await expect(page.locator('.image-viewer')).toHaveCount(0)

  // The close button.
  await thumb.click()
  await page.getByRole('button', { name: 'Close image' }).click()
  await expect(page.locator('.image-viewer')).toHaveCount(0)
})
