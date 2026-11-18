#!/usr/bin/env bun
// Takes the website's screenshots from the app's demo mode and writes them to public/shots/ as
// WebP, in light and dark. Start a demo first (see "Website" in the README), then run
// `bun run capture` from website/. BASE selects another demo address.
//
// The screenshots use the project's own pixel-art profile pictures and illustration, and the name
// "Robin" for the signed-in person: the demo's photos have no recorded license, and its signed-in
// name belongs to a real person. Install a color emoji font first, or emoji draw as empty boxes.
import { chromium } from 'playwright'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { randomUUID } from 'node:crypto'

const base = (process.env.BASE ?? 'http://127.0.0.1:4173').replace(/\/$/, '')
const out = new URL('../public/shots/', import.meta.url)
const DESKTOP = { width: 1440, height: 900 }
const PHONE = { width: 390, height: 844 }

/** Demo people with a photo get one of the project's pixel-art pictures instead. */
const avatars = { mika: 37, jules: 38, noah: 33, sage: 34 }
const appearance = (mode) => ({
  settings: { mode, schemeDark: 'hibiki-dark', schemeLight: 'hibiki-light' },
  revision: null,
  dirty: false,
  sync: false,
})

/** A road through red rock at sunset, drawn for the website in place of the demo photo. */
const illustration = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 1440">
<defs>
<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#3b2a5c"/><stop offset=".55" stop-color="#f08a6c"/><stop offset="1" stop-color="#ffd29a"/></linearGradient>
<linearGradient id="ground" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#b8553a"/><stop offset="1" stop-color="#6e2a22"/></linearGradient>
</defs>
<rect width="960" height="1440" fill="url(#sky)"/>
<circle cx="610" cy="560" r="96" fill="#ffe7b8"/>
<path d="M0 640 120 520 230 590 360 450 470 560 560 500 700 610 820 470 960 560V900H0Z" fill="#9a4a5c"/>
<path d="M0 760 140 650 260 720 390 600 520 700 640 640 760 730 880 640 960 690V1000H0Z" fill="#c2563f"/>
<path d="M0 860 180 780 320 840 480 770 640 850 800 790 960 840V1440H0Z" fill="url(#ground)"/>
<path d="M455 860C470 900 380 960 360 1060 330 1200 250 1340 160 1440H800C700 1340 600 1210 560 1060 530 950 500 900 505 860Z" fill="#2c2628"/>
<path d="M480 870C485 920 450 980 440 1060 425 1180 400 1320 380 1440" stroke="#f6c453" stroke-width="10" fill="none" stroke-dasharray="40 34"/>
</svg>`

async function toWebp(page, png) {
  const data = await page.evaluate(async (b64) => {
    const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))
    const bitmap = await createImageBitmap(new Blob([bytes], { type: 'image/png' }))
    const canvas = new OffscreenCanvas(bitmap.width, bitmap.height)
    canvas.getContext('2d').drawImage(bitmap, 0, 0)
    const blob = await canvas.convertToBlob({ type: 'image/webp', quality: 0.86 })
    const buffer = new Uint8Array(await blob.arrayBuffer())
    let text = ''
    for (let i = 0; i < buffer.length; i += 0x8000)
      text += String.fromCharCode(...buffer.subarray(i, i + 0x8000))
    return btoa(text)
  }, png.toString('base64'))
  return Buffer.from(data, 'base64')
}

async function session(browser, mode, viewport) {
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 2,
    colorScheme: mode,
    reducedMotion: 'reduce',
    serviceWorkers: 'block',
    timezoneId: 'UTC',
    locale: 'en-US',
  })
  await context.addInitScript(
    (value) => {
      localStorage.setItem('ircord.appearance', value)
    },
    JSON.stringify(appearance('system')),
  )
  for (const [name, number] of Object.entries(avatars))
    await context.route(`${base}/avatars/${name}.jpg`, (route) =>
      route.continue({
        url: `${base}/avatars/defaults/avatar-${String(number).padStart(3, '0')}.png`,
      }),
    )
  await context.route(`${base}/media/weekend.jpg`, (route) =>
    route.fulfill({ contentType: 'image/svg+xml', body: illustration }),
  )
  // The demo's signed-in name is a real person's; show a neutral one.
  await context.route(`${base}/api/**`, async (route) => {
    const response = await route.fetch()
    const type = response.headers()['content-type'] ?? ''
    if (!type.includes('json')) return route.fulfill({ response })
    const body = (await response.text())
      .replaceAll('karbowiak', 'robin')
      .replaceAll('Karbowiak', 'Robin')
      // Show the demo network under the project's own name.
      .replaceAll('Libera Chat', 'IRCord')
      .replaceAll('Libera.Chat', 'IRCord')
      .replace(/("name":\s*"IRCord"[^{}]*?"initials":\s*)"L"/g, '$1"I"')
    return route.fulfill({ response, body })
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `site${Date.now().toString(36)}`, password: randomUUID() },
  })
  if (signup.status() !== 201) throw new Error(`Demo sign-up failed: ${signup.status()}`)
  const page = await context.newPage()
  page.setDefaultTimeout(10_000)
  return { context, page }
}

async function settle(page) {
  await page.evaluate(() => document.fonts.ready)
  await page.waitForLoadState('networkidle').catch(() => {})
  await page.waitForTimeout(600)
}

async function dismissNotice(page) {
  const dismiss = page.getByRole('button', { name: 'Dismiss' })
  if (await dismiss.count()) await dismiss.first().click()
}

/** Each picture's pixel size, without its -light or -dark suffix, for the pages' layout. */
const sizes = {}

async function save(page, name, shot) {
  const file = new URL(`${name}.webp`, out)
  await writeFile(file, await toWebp(page, shot))
  // A PNG's IHDR chunk holds the width and height.
  sizes[name.replace(/-(light|dark)$/, '')] = {
    width: shot.readUInt32BE(16),
    height: shot.readUInt32BE(20),
  }
  console.log(`Wrote shots/${name}.webp`)
}

async function openGeneral(page) {
  await page.goto(base)
  await page.waitForURL(/\/channel\/general/)
  await dismissNotice(page)
  await settle(page)
}

/** The union of several elements' boxes, padded and kept inside the viewport. */
async function around(page, locators, pad = 16) {
  const boxes = []
  for (const locator of locators) boxes.push(await locator.boundingBox())
  const view = page.viewportSize()
  const x = Math.max(0, Math.min(...boxes.map((b) => b.x)) - pad)
  const y = Math.max(0, Math.min(...boxes.map((b) => b.y)) - pad)
  const right = Math.min(view.width, Math.max(...boxes.map((b) => b.x + b.width)) + pad)
  const bottom = Math.min(view.height, Math.max(...boxes.map((b) => b.y + b.height)) + pad)
  return { x, y, width: right - x, height: bottom - y }
}

async function desktop(browser, mode) {
  const { context, page } = await session(browser, mode, DESKTOP)
  await openGeneral(page)
  await save(page, `channel-desktop-${mode}`, await page.screenshot())

  // A crop of the photo with its conversation.
  const photo = page
    .locator('.message-row')
    .filter({ has: page.locator('img[alt^="A quiet road"]') })
  await photo.scrollIntoViewIfNeeded()
  await photo.evaluate((row) => row.scrollIntoView({ block: 'start' }))
  await settle(page)
  const timeline = page.locator('.message-row')
  const rows = await timeline.all()
  const index = await photo.evaluate((row) =>
    [...document.querySelectorAll('.message-row')].indexOf(row),
  )
  const media = await around(page, rows.slice(index, index + 2))
  await save(page, `media-${mode}`, await page.screenshot({ clip: { ...media, width: 720 } }))

  // Reactions and formatting at the end of the conversation, in a narrower window so the
  // messages wrap inside the crop.
  await page.setViewportSize({ width: 1100, height: DESKTOP.height })
  await page.locator('.message-row').last().scrollIntoViewIfNeeded()
  await settle(page)
  const last = await page.locator('.message-row').all()
  const reactions = await around(page, last.slice(-4))
  await save(page, `reactions-${mode}`, await page.screenshot({ clip: reactions }))
  await page.setViewportSize(DESKTOP)

  // A profile card.
  await page
    .locator('.member-list, [aria-label="Members"]')
    .getByText('Mika', { exact: true })
    .first()
    .click()
  const card = page.locator('[role="dialog"]').last()
  await card.waitFor()
  await settle(page)
  await save(page, `profile-${mode}`, await card.screenshot())
  await page.keyboard.press('Escape')

  for (const [name, path] of [
    ['search', '/search?q=desk'],
    ['inbox', '/inbox'],
  ]) {
    await page.goto(base + path)
    await page.locator('.workspace-page h1').waitFor()
    await dismissNotice(page)
    await settle(page)
    const panel = await page.locator('.workspace-page').boundingBox()
    await save(
      page,
      `${name}-${mode}`,
      await page.screenshot({
        clip: { x: panel.x, y: panel.y, width: Math.min(panel.width, 760), height: 475 },
      }),
    )
  }

  // Appearance settings.
  await openGeneral(page)
  // Settings opens on Appearance.
  await page.getByRole('button', { name: 'User settings' }).click()
  await page.getByRole('dialog').first().waitFor()
  await settle(page)
  await save(page, `appearance-${mode}`, await page.getByRole('dialog').first().screenshot())
  await context.close()
}

async function phone(browser, mode) {
  const { context, page } = await session(browser, mode, PHONE)
  await openGeneral(page)
  await save(page, `channel-phone-${mode}`, await page.screenshot())
  await context.close()
}

/** The 1200×630 picture link previews show for the website (public/og.png). */
async function socialCard(browser) {
  const page = await browser.newPage({ viewport: { width: 1200, height: 630 } })
  const font = await readFile(
    createRequire(import.meta.url).resolve(
      '@fontsource-variable/geist/files/geist-latin-wght-normal.woff2',
    ),
  )
  const logo = await readFile(new URL('../public/favicon.svg', import.meta.url))
  const shot = await readFile(new URL('channel-desktop-dark.webp', out))
  const data = (type, buffer) => `data:${type};base64,${buffer.toString('base64')}`
  await page.setContent(`<style>
@font-face { font-family: Geist; src: url(${data('font/woff2', font)}); font-weight: 100 900; }
body { margin: 0; width: 1200px; height: 630px; overflow: hidden; font-family: Geist;
  color: #f4f0f3; background: radial-gradient(70% 70% at 30% 0%, #f43f5e33, transparent 70%), #111013; }
.text { position: absolute; left: 72px; top: 72px; width: 520px; }
.brand { display: flex; align-items: center; gap: 16px; font-size: 34px; font-weight: 700; }
h1 { margin: 56px 0 0; font-size: 68px; line-height: 1.04; letter-spacing: -0.03em; }
p { margin: 24px 0 0; font-size: 26px; line-height: 1.4; color: #a89ea6; }
.shot { position: absolute; left: 640px; top: 90px; width: 900px; border-radius: 22px;
  border: 2px solid #2d2730; box-shadow: 0 30px 80px #0008; }
</style>
<div class="text"><div class="brand"><img src="${data('image/svg+xml', logo)}" width="56" height="56" alt="">IRCord</div>
<h1>IRC that stays connected.</h1><p>A web client and a bouncer in one.</p></div>
<img class="shot" src="${data('image/webp', shot)}" alt="">`)
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: new URL('../public/og.png', import.meta.url).pathname })
  console.log('Wrote og.png')
  await page.close()
}

await mkdir(out, { recursive: true })
const browser = await chromium.launch()
try {
  for (const mode of ['light', 'dark']) {
    await desktop(browser, mode)
    await phone(browser, mode)
  }
  await socialCard(browser)
} finally {
  await browser.close()
}
await writeFile(
  new URL('../app/shots.json', import.meta.url),
  `${JSON.stringify(sizes, null, 2)}\n`,
)
console.log('Wrote app/shots.json')
