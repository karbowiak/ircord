import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

// A 1×1 PNG, served for every remote image so nothing leaves the test machine.
const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=',
  'base64',
)
const violations = async (page: Page) =>
  (
    await new AxeBuilder({ page })
      .include('.timeline-scroll')
      .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
      .analyze()
  ).violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }))

/**
 * Serves remote requests and returns the hosts requested. Like i.ytimg.com, a missing thumbnail
 * answers 404 with a 120×90 placeholder image, which browsers load without an error event.
 */
async function serveRemote(page: Page) {
  const hosts: string[] = []
  const placeholder = Buffer.from(
    (
      await page.evaluate(() => {
        const canvas = document.createElement('canvas')
        canvas.width = 120
        canvas.height = 90
        return canvas.toDataURL('image/png')
      })
    ).split(',')[1]!,
    'base64',
  )
  await page.route(
    (url) => url.protocol === 'https:' && url.hostname !== '127.0.0.1',
    (route) => {
      const url = new URL(route.request().url())
      hosts.push(url.hostname)
      if (url.hostname === 'www.youtube-nocookie.com')
        return route.fulfill({ contentType: 'text/html', body: '<title>player</title>' })
      if (url.pathname.includes('missing'))
        return route.fulfill({ status: 404, contentType: 'image/png', body: placeholder })
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
async function serverPreviews(page: Page) {
  await page
    .locator('.conversation-sidebar .conversation-link')
    .filter({ hasText: 'general' })
    .click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Channel settings…' }).click()
  await page.getByRole('switch', { name: 'Server link previews' }).click()
  await expect(page.getByRole('switch', { name: 'Server link previews' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await page.keyboard.press('Escape')
}
/** The extras of the first message containing `text` (the demo echo bot repeats each one). */
const extras = (page: Page, text: string) =>
  page
    .locator('.message-row')
    .filter({ has: page.locator('.message-text', { hasText: text }) })
    .first()
    .locator('.message-extras')

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('YouTube links become one click-to-play player per video', async ({ page }) => {
  const hosts = await serveRemote(page)
  await send(
    page,
    'watch https://youtu.be/dQw4w9WgXcQ?t=90 https://www.youtube.com/watch?v=dQw4w9WgXcQ',
  )
  const own = extras(page, 'watch')
  const card = own.locator('.embed-card')
  await expect(card).toHaveCount(1)
  await expect(card).toContainText('YouTube video')
  await expect(card.locator('img')).toHaveAttribute(
    'src',
    'https://i.ytimg.com/vi/dQw4w9WgXcQ/hqdefault.jpg',
  )
  // The 16:9 frame is reserved before the thumbnail loads.
  const box = (await card.locator('.embed-frame').boundingBox())!
  expect(Math.abs(box.width / box.height - 16 / 9)).toBeLessThan(0.02)
  // Nothing from the player loads until someone presses play.
  expect(hosts).not.toContain('www.youtube-nocookie.com')
  await expect(card.locator('iframe')).toHaveCount(0)
  expect(await violations(page)).toEqual([])

  await card.getByRole('button', { name: 'Play YouTube video from YouTube' }).click()
  const player = card.locator('iframe')
  await expect(player).toHaveAttribute(
    'src',
    'https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?autoplay=1&rel=0&playsinline=1&start=90',
  )
  await expect(player).toHaveAttribute('sandbox', /allow-scripts/)
  await expect(card.getByRole('link', { name: 'YouTube video' })).toHaveAttribute(
    'href',
    'https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=90s',
  )
})

test('Shorts use a vertical frame and broken thumbnails leave a link', async ({ page }) => {
  await serveRemote(page)
  await send(page, 'short https://www.youtube.com/shorts/abcdefghijk')
  const frame = extras(page, 'short').locator('.embed-card.is-short .embed-frame')
  await expect(frame).toBeVisible()
  const box = (await frame.boundingBox())!
  expect(box.height).toBeGreaterThan(box.width)
  // The fixture answers 404 with YouTube's placeholder image for this thumbnail.
  await send(page, 'gone https://youtu.be/missing0000')
  await expect(
    extras(page, 'gone').getByRole('link', { name: 'Video unavailable · YouTube' }),
  ).toHaveAttribute('href', 'https://www.youtube.com/watch?v=missing0000')
})

test('server previews give YouTube its title, and respect disabled embedding', async ({ page }) => {
  await serveRemote(page)
  await serverPreviews(page)
  await send(page, 'tune https://www.youtube.com/watch?v=dQw4w9WgXcQ')
  const own = extras(page, 'tune')
  await expect(own.locator('.embed-card')).toContainText('Demo video dQw4w9WgXcQ')
  await expect(own.locator('.embed-card')).toContainText('YouTube · IRCord Demo')
  // The server's card merges into the player instead of showing twice.
  await expect(own.locator('.link-card')).toHaveCount(0)
  await expect(own.locator('.embed-card')).toHaveCount(1)

  await send(page, 'nope https://youtu.be/blockedVid1')
  const blocked = extras(page, 'nope').locator('.embed-card')
  await expect(
    blocked.getByRole('link', { name: /Playback is disabled outside YouTube/ }),
  ).toBeVisible()
  await expect(blocked.getByRole('button', { name: /^Play / })).toHaveCount(0)
})

test('the server marks deleted videos unavailable and merges stored YouTube cards', async ({
  page,
}) => {
  const hosts = await serveRemote(page)
  await serverPreviews(page)
  await send(page, 'deleted https://youtu.be/goneVideo01')
  const gone = extras(page, 'deleted')
  await expect(gone.getByRole('link', { name: 'Video unavailable · YouTube' })).toHaveAttribute(
    'href',
    'https://www.youtube.com/watch?v=goneVideo01',
  )
  await expect(gone.locator('.embed-card, .link-card')).toHaveCount(0)
  await expect(gone.getByRole('button', { name: /^Play / })).toHaveCount(0)

  // A preview stored before typed embeds is a plain card for the YouTube URL: it becomes the
  // player's title instead of a second card.
  await send(page, 'oldlink https://youtu.be/legacyVid01')
  const old = extras(page, 'oldlink')
  await expect(old.locator('.embed-card')).toContainText('Stored video title')
  await expect(old.locator('.embed-card')).toHaveCount(1)
  await expect(old.locator('.link-card')).toHaveCount(0)
  expect(hosts).not.toContain('www.youtube-nocookie.com')
})

test('with link previews off, YouTube and media links load nothing', async ({ page }) => {
  const hosts = await serveRemote(page)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'Messages & media', exact: true })
    .click()
  await page
    .getByRole('radiogroup', { name: 'Link previews' })
    .getByRole('radio', { name: 'Off' })
    .click()
  await page.keyboard.press('Escape')
  await serverPreviews(page)
  await send(
    page,
    'quiet https://youtu.be/dQw4w9WgXcQ https://i.imgur.com/a.png https://i.imgur.com/b.png',
  )
  const own = extras(page, 'quiet')
  // Only the server's text card remains.
  await expect(own.locator('.link-card')).toContainText('Demo video dQw4w9WgXcQ')
  await expect(own.locator('.embed-card, .message-gallery, img, iframe')).toHaveCount(0)
  expect(hosts).toEqual([])
})

test('several images form a gallery that the viewer pages through', async ({ page }) => {
  await serveRemote(page)
  const urls = [1, 2, 3, 4, 5].map((n) => `https://i.imgur.com/photo${n}.png`)
  await send(page, `holidaypics ${urls.join(' ')}`)
  const gallery = extras(page, 'holidaypics').getByRole('group', { name: '5 images' })
  await expect(gallery.getByRole('button')).toHaveCount(4)
  await expect(gallery.locator('.gallery-more')).toHaveText('+1')
  const box = (await gallery.boundingBox())!
  expect(box.height).toBe(300)
  expect(await violations(page)).toEqual([])

  await gallery.getByRole('button', { name: 'View image 2 of 5 · i.imgur.com' }).click()
  const viewer = page.getByRole('dialog')
  await expect(viewer).toContainText('2 of 5')
  await expect(viewer.locator('.image-viewer-stage img')).toHaveAttribute('src', urls[1]!)
  await page.keyboard.press('ArrowRight')
  await expect(viewer).toContainText('3 of 5')
  await expect(viewer.locator('.image-viewer-stage img')).toHaveAttribute('src', urls[2]!)
  await viewer.getByRole('button', { name: 'Previous image' }).click()
  await viewer.getByRole('button', { name: 'Previous image' }).click()
  await page.keyboard.press('ArrowLeft')
  // Paging wraps around.
  await expect(viewer).toContainText('5 of 5')
  await expect(viewer.locator('.image-viewer-stage img')).toHaveAttribute('src', urls[4]!)
  await page.keyboard.press('Escape')
  await expect(page.locator('.image-viewer')).toHaveCount(0)
})

test('article cards show the lead image in its shape, and cope without one', async ({ page }) => {
  await serveRemote(page)
  await serverPreviews(page)
  await send(page, 'story https://paper.example/news/launch')
  const card = extras(page, 'story').locator('.link-card.is-article')
  await expect(card).toContainText('paper.example News')
  await expect(card).toContainText('Headline: launch')
  await expect(card).toContainText('A summary of the story')
  expect(
    await card.locator('.link-card-image').evaluate((el) => getComputedStyle(el).aspectRatio),
  ).toBe('1200 / 630')

  await send(page, 'bare https://paper.example/news/bare')
  const bare = extras(page, 'bare').locator('.link-card.is-article')
  await expect(bare).toContainText('Headline: bare')
  await expect(bare.locator('.link-card-image, .link-card-text > span')).toHaveCount(0)
})

test('album pages show their images as a gallery', async ({ page }) => {
  await serveRemote(page)
  await serverPreviews(page)
  await send(page, 'album https://photos.example/album/holiday')
  const own = extras(page, 'album')
  await expect(own.locator('.link-card')).toContainText('Holiday album')
  const gallery = own.getByRole('group', { name: '5 images from photos.example' })
  await gallery.getByRole('button', { name: /^View image 4 of 5/ }).click()
  await expect(page.getByRole('dialog')).toContainText('4 of 5')
})

test('audio links get a player that loads nothing until played', async ({ page }) => {
  await serveRemote(page)
  await send(page, 'listen https://cdn.discordapp.com/attachments/song.mp3')
  const audio = extras(page, 'listen').locator('audio')
  await expect(audio).toHaveAttribute('preload', 'none')
  await expect(audio).toHaveAttribute('controls', '')
  await expect(audio).toHaveAccessibleName('Audio song.mp3 from cdn.discordapp.com')
  await expect(audio).toHaveJSProperty('paused', true)
})

test.describe('on a phone', () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })

  test('embeds fit the screen and a swipe pages the gallery viewer', async ({ page }) => {
    await serveRemote(page)
    await send(
      page,
      'phonepics https://i.imgur.com/p1.png https://i.imgur.com/p2.png https://i.imgur.com/p3.png',
    )
    await send(page, 'phoneclip https://youtu.be/dQw4w9WgXcQ')
    await send(page, 'phonesong https://cdn.discordapp.com/a/song.mp3')
    for (const selector of ['.message-gallery', '.embed-card', '.message-audio']) {
      const box = (await page.locator(selector).last().boundingBox())!
      expect(box.x + box.width, selector).toBeLessThanOrEqual(390)
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390)

    await extras(page, 'phonepics')
      .getByRole('button', { name: 'View image 1 of 3 · i.imgur.com' })
      .tap()
    const viewer = page.getByRole('dialog')
    await expect(viewer).toContainText('1 of 3')
    const stage = (await viewer.locator('.image-viewer-stage').boundingBox())!
    const y = stage.y + stage.height / 2
    const cdp = await page.context().newCDPSession(page)
    const touch = (type: string, x: number) =>
      cdp.send('Input.dispatchTouchEvent', {
        type,
        touchPoints: type === 'touchEnd' ? [] : [{ x, y }],
      })
    await touch('touchStart', 300)
    for (const x of [260, 200, 140, 100]) await touch('touchMove', x)
    await touch('touchEnd', 100)
    await expect(viewer).toContainText('2 of 3')
    await expect(page.locator('.image-viewer')).toBeVisible()
  })
})

test('channel operators add trusted media hosts that embed in that channel only', async ({
  page,
}) => {
  const hosts = await serveRemote(page)
  await page.locator('.channel-name-button').click()
  const shared = page
    .getByRole('dialog', { name: '#general settings' })
    .getByRole('region', { name: 'For everyone in #general' })
  const field = shared.getByRole('textbox', { name: 'Extra trusted media hosts' })
  // An invalid entry is refused with the reason; nothing is saved and the draft is kept to fix.
  await field.fill('images.example.com\nhttps://other.example.com/cats')
  await shared.getByRole('button', { name: 'Save hosts' }).click()
  await expect(page.getByText('must be a bare public host name')).toBeVisible()
  await expect(field).toHaveValue('images.example.com\nhttps://other.example.com/cats')
  await expect(shared.getByRole('button', { name: 'Save hosts' })).toBeEnabled()
  await page.keyboard.press('Escape')
  await page.locator('.channel-name-button').click()
  await expect(field).toHaveValue('')
  await field.fill('Images.Example.com images.example.com')
  await shared.getByRole('button', { name: 'Save hosts' }).click()
  await expect(field).toHaveValue('images.example.com')
  await expect(shared.getByRole('button', { name: 'Save hosts' })).toBeDisabled()
  await page.keyboard.press('Escape')

  await send(page, 'botpics https://images.example.com/a.png https://other.example.com/b.png')
  const own = extras(page, 'botpics')
  await expect(own.locator('img')).toHaveCount(1)
  await expect(own.locator('img')).toHaveAttribute('src', 'https://images.example.com/a.png')
  expect(hosts).not.toContain('other.example.com')

  // Another channel does not share #general's list, and its non-operators cannot change one.
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  const input = page.getByRole('combobox', { name: 'Message #development' })
  await input.fill('devpics https://images.example.com/c.png')
  await input.press('Enter')
  await expect(page.locator('.message-text', { hasText: 'devpics' }).first()).toBeVisible()
  await expect(extras(page, 'devpics').locator('img')).toHaveCount(0)
  await page.locator('.channel-name-button').click()
  const readOnly = page
    .getByRole('dialog', { name: '#development settings' })
    .getByRole('region', { name: 'For everyone in #development' })
  await expect(readOnly.getByRole('textbox', { name: 'Extra trusted media hosts' })).toHaveAttribute(
    'readonly',
    '',
  )
  await expect(readOnly.getByRole('button', { name: 'Save hosts' })).toHaveCount(0)
})
