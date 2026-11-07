import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

/** A 1×1 GIF and a WebM recorded in the browser, served from a trusted preview host. */
const gif = Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64')
let webm: Buffer | undefined
/** About six seconds long, so a preview cannot end by itself while a test watches it. */
async function clip(page: Page) {
  webm ??= Buffer.from(
    await page.evaluate(async () => {
      const canvas = document.createElement('canvas')
      canvas.width = 32
      canvas.height = 24
      const context = canvas.getContext('2d')!
      const recorder = new MediaRecorder(canvas.captureStream(10), { mimeType: 'video/webm' })
      const chunks: Blob[] = []
      recorder.ondataavailable = (e) => chunks.push(e.data)
      const done = new Promise((resolve) => (recorder.onstop = resolve))
      recorder.start()
      for (let i = 0; i < 60; i++) {
        context.fillStyle = `hsl(${i * 6} 80% 50%)`
        context.fillRect(0, 0, 32, 24)
        await new Promise((r) => setTimeout(r, 100))
      }
      recorder.stop()
      await done
      const bytes = new Uint8Array(await new Blob(chunks).arrayBuffer())
      let binary = ''
      for (const b of bytes) binary += String.fromCharCode(b)
      return btoa(binary)
    }),
    'base64',
  )
  return webm
}
async function serveMedia(page: Page) {
  const video = await clip(page)
  await page.route('https://media.discordapp.net/**', (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('.gif')) return route.fulfill({ body: gif, contentType: 'image/gif' })
    if (path.endsWith('.webm')) return route.fulfill({ body: video, contentType: 'video/webm' })
    return route.fulfill({ status: 404 })
  })
  await page.route('https://media.giphy.com/**', (route) =>
    route.fulfill({ body: gif, contentType: 'image/gif' }),
  )
}
async function send(page: Page, text: string) {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill(text)
  await input.press('Enter')
}
const state = (v: HTMLVideoElement) => ({ paused: v.paused, ended: v.ended, muted: v.muted })
async function turnOnAutoplay(page: Page) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Notifications', exact: true }).click()
  await page.getByRole('switch', { name: 'Autoplay GIFs and videos' }).click()
  await page.keyboard.press('Escape')
}

test('linked GIFs, animated WebP and videos are labelled and hold still until previewed', async ({
  page,
}) => {
  await signIn(page)
  await serveMedia(page)

  await send(page, 'look https://media.discordapp.net/a/dance.gif')
  const still = page.getByRole('button', { name: 'View GIF from media.discordapp.net' }).first()
  await expect(still.locator('.media-kind')).toHaveText('GIF')
  await expect(still).toHaveAttribute('aria-busy', 'false')
  await expect(still.locator('canvas.media-still')).toBeVisible()
  await expect(still.locator('img')).toHaveCSS('visibility', 'hidden')
  await still.hover()
  await expect(still.locator('canvas.media-still')).toBeHidden()
  await expect(still.locator('img')).toHaveCSS('visibility', 'visible')
  await page.mouse.move(0, 0)
  await expect(still.locator('canvas.media-still')).toBeVisible()

  // GIF hosts serve animated WebP as their GIFs: same still frame and label.
  await send(page, 'webp https://media.giphy.com/media/abc/giphy.webp')
  const webp = page.getByRole('button', { name: 'View GIF from media.giphy.com' }).first()
  await expect(webp.locator('.media-kind')).toHaveText('GIF')
  await expect(webp.locator('canvas.media-still')).toBeVisible()

  await send(page, 'clip https://media.discordapp.net/a/clip.webm')
  // .first(): the demo Echo bot repeats each message, adding a second copy below.
  const frame = page.locator('.media-frame:has(video)').first()
  const video = frame.locator('video')
  await expect(frame.locator('.media-kind')).toHaveText('Video')
  await expect(frame).toHaveAttribute('aria-busy', 'false')
  expect(await video.evaluate(state)).toMatchObject({ paused: true })
  await frame.hover()
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: false, muted: true })
  await page.mouse.move(0, 0)
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: true, ended: false })

  // Keyboard focus previews the video as well.
  await page.keyboard.press('Shift')
  await video.focus()
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: false })
  await video.blur()
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: true, ended: false })

  // Once the user unmutes, the video is theirs: leaving or hovering again changes nothing.
  await frame.hover()
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: false })
  // Unmute inside a real key press with the pointer still on the video, as the native mute
  // button would; the browser pauses media that a script unmutes without a user gesture.
  await video.evaluate((v: HTMLVideoElement) =>
    document.addEventListener('keydown', () => (v.muted = false), { once: true, capture: true }),
  )
  await page.keyboard.press('Control')
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: false, muted: false })
  await page.mouse.move(0, 0)
  await page.waitForTimeout(300)
  expect(await video.evaluate(state)).toMatchObject({ paused: false, muted: false })
  await frame.hover()
  await page.waitForTimeout(300)
  expect(await video.evaluate(state)).toMatchObject({ paused: false, muted: false })

  // A broken video leaves a link instead of vanishing.
  await send(page, 'gone https://media.discordapp.net/a/missing.mp4')
  await expect(
    page.getByRole('link', { name: 'Video unavailable · media.discordapp.net' }),
  ).toBeVisible()
})

test('uploaded images are checked for animation and broken uploads leave a link', async ({
  page,
}) => {
  await signIn(page)
  const upload = page.locator('input[type=file][multiple]')
  const sendButton = page.getByRole('button', { name: 'Send message', exact: true })

  await upload.setInputFiles({ name: 'dance.gif', mimeType: 'image/gif', buffer: gif })
  await sendButton.click()
  const declared = page.getByRole('button', { name: 'View dance.gif' }).last()
  await expect(declared.locator('.media-kind')).toHaveText('GIF')
  await expect(declared.locator('canvas.media-still')).toBeVisible()

  // Declared as a still PNG, but the bytes are a GIF: the byte check labels it.
  await upload.setInputFiles({ name: 'sneaky.png', mimeType: 'image/png', buffer: gif })
  await sendButton.click()
  const sniffed = page.getByRole('button', { name: 'View sneaky.png' }).last()
  await expect(sniffed.locator('.media-kind')).toHaveText('GIF')

  await upload.setInputFiles('public/avatars/noah.jpg')
  await sendButton.click()
  const photo = page.getByRole('button', { name: 'View noah.jpg' }).last()
  await expect(photo).toHaveAttribute('aria-busy', 'false')
  await expect(photo.locator('.media-kind')).toHaveCount(0)

  // An upload that cannot be decoded leaves a link, and nothing stays busy.
  await upload.setInputFiles({
    name: 'broken.gif',
    mimeType: 'image/gif',
    buffer: Buffer.from('not an image'),
  })
  await sendButton.click()
  await expect(
    page.getByRole('link', { name: 'Image unavailable · broken.gif' }).last(),
  ).toBeVisible()
})

test('on touch screens the first tap plays a GIF, the next opens it, and a tap elsewhere stops it', async ({
  browser,
}) => {
  const context = await browser.newContext({
    hasTouch: true,
    viewport: { width: 1280, height: 900 },
  })
  const page = await context.newPage()
  await signIn(page)
  await serveMedia(page)
  await send(page, 'look https://media.discordapp.net/a/dance.gif')
  const still = page.getByRole('button', { name: 'View GIF from media.discordapp.net' }).first()
  await expect(still.locator('canvas.media-still')).toBeVisible()
  await still.tap()
  await expect(still.locator('canvas.media-still')).toBeHidden()
  await expect(page.locator('.dialog-image')).toBeHidden()
  await page.locator('.timeline-scroll').tap({ position: { x: 600, y: 40 } })
  await expect(still.locator('canvas.media-still')).toBeVisible()
  await still.tap()
  await still.tap()
  await expect(page.locator('.dialog-image')).toBeVisible()
  await context.close()
})

test('with autoplay on, videos play muted only while near the viewport', async ({ page }) => {
  await signIn(page)
  await serveMedia(page)
  await turnOnAutoplay(page)
  await send(page, 'clip https://media.discordapp.net/a/clip.webm')
  const video = page.locator('.media-frame video').first()
  await expect.poll(() => video.evaluate(state)).toMatchObject({ paused: false, muted: true })

  const scroller = page.locator('.timeline-scroll')
  await scroller.evaluate((el) => (el.scrollTop = 0))
  // Scrolled away it pauses, or the timeline unmounts its row altogether.
  await expect
    .poll(async () => (await video.count()) === 0 || (await video.evaluate((v) => v.paused)))
    .toBe(true)
  await scroller.evaluate((el) => (el.scrollTop = el.scrollHeight))
  await expect
    .poll(() => page.locator('.media-frame video').first().evaluate(state))
    .toMatchObject({ paused: false })
})
