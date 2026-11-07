import { chromium } from '@playwright/test'
import { mkdir } from 'node:fs/promises'

const browser = await chromium.launch()
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await page.goto('http://127.0.0.1:3000/')
  async function settleImages() {
    await page.locator('.expression-body').evaluate(async (root) => {
      const clip = root.getBoundingClientRect()
      await Promise.all(
        Array.from(root.querySelectorAll('img'))
          .filter((img) => {
            const box = img.getBoundingClientRect()
            return box.bottom > clip.top && box.top < clip.bottom
          })
          .map((img) => img.decode().catch(() => {})),
      )
    })
  }
  await page.getByRole('textbox', { name: 'Message #general' }).waitFor()
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.waitForFunction(
    () => document.querySelector('.gif-category') || document.querySelector('.picker-empty strong'),
  )
  const problem = (await page.locator('.picker-empty').count())
    ? await page.locator('.picker-empty').textContent()
    : null
  if (problem) throw new Error(problem)
  await mkdir('.impeccable/review', { recursive: true })
  await settleImages()
  await page.screenshot({ path: '.impeccable/review/gif-discover.png', animations: 'disabled' })
  await page.getByRole('textbox', { name: 'Search Klipy GIFs' }).fill('hello')
  await page.locator('.gif-select').first().waitFor({ timeout: 20000 })
  console.log(
    JSON.stringify({
      liveGifResults: await page.locator('.gif-select').count(),
      categories: await page.locator('.gif-category').count(),
    }),
  )
  await settleImages()
  await page.screenshot({ path: '.impeccable/review/gif-search.png', animations: 'disabled' })
  await page.locator('.gif-save').first().click()
  await page.getByRole('button', { name: 'Close picker' }).click()
  await page.reload()
  await page.getByRole('textbox', { name: 'Message #general' }).waitFor()
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.getByRole('button', { name: /Favorites.*saved GIFs/ }).click()
  await page.locator('.gif-select').first().waitFor({ timeout: 20000 })
  await settleImages()
  await page.screenshot({ path: '.impeccable/review/gif-favorites.png', animations: 'disabled' })
  if ((await page.locator('.gif-save').first().getAttribute('aria-pressed')) !== 'true')
    throw new Error('Saved GIF did not retain its favorite state.')
  await page.locator('.gif-select').first().click()
  await page.waitForFunction(() => document.querySelectorAll('.message-gif').length >= 2)
  await page.getByRole('button', { name: 'Choose an emoji', exact: true }).click()
  await page.getByRole('button', { name: 'Smileys & emotion', exact: true }).click()
  await page.locator('.emoji-choice').first().waitFor()
  await page.screenshot({ path: '.impeccable/review/emoji-standard.png', animations: 'disabled' })
  await page.getByRole('button', { name: '7TV global emotes', exact: true }).click()
  await page.getByRole('button', { name: 'Insert PETPET', exact: true }).waitFor({ timeout: 15000 })
  await page.locator('.emoji-choice img[alt=":PETPET:"]').evaluate((img) => img.decode())
  await settleImages()
  await page.screenshot({ path: '.impeccable/review/emoji-custom.png', animations: 'disabled' })
  await page.getByRole('button', { name: 'Insert PETPET', exact: true }).click()
  await page.getByRole('textbox', { name: 'Message #general' }).press('Enter')
  await page.waitForFunction(() => document.querySelectorAll('.inline-custom-emoji').length >= 2)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.locator('.gif-category').first().waitFor()
  await settleImages()
  await page.screenshot({ path: '.impeccable/review/gif-mobile.png', animations: 'disabled' })
  await page.getByRole('button', { name: 'Close picker' }).click()

  await page.getByRole('button', { name: 'Choose an emoji', exact: true }).click()
  await page.getByRole('textbox', { name: 'Search emoji' }).fill('wave')
  await page.getByRole('button', { name: 'Insert waving hand', exact: true }).waitFor()
  await settleImages()
  await page.screenshot({ path: '.impeccable/review/emoji-mobile.png', animations: 'disabled' })
  console.log(
    'PASS: live Klipy browser search, saved favorite restoration, favorites revalidation, GIF echo, 7TV custom emote rendering, and mobile emoji search.',
  )
} catch (error) {
  const safe = String(error?.message ?? error).replace(
    /(api\/v1\/)[A-Za-z0-9]{20,}/g,
    '$1[app-key]',
  )
  console.error(safe)
  process.exitCode = 1
} finally {
  await browser.close()
}
