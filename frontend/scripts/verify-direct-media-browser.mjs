// Disposable Nuxt demo, fixture messages and generated media; no real-user mutations.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { spawn, spawnSync } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const temporary = await mkdtemp(join(tmpdir(), 'ircord-media-fixture-'))
const videoFile = join(temporary, 'fixture.webm')
const audioFile = join(temporary, 'fixture.wav')
function ffmpeg(args) {
  const result = spawnSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', ...args], {
    encoding: 'utf8',
  })
  assert.equal(result.status, 0, result.stderr || result.error?.message)
}
try {
  ffmpeg([
    '-f',
    'lavfi',
    '-i',
    'color=c=0x5b7598:s=160x90:r=5:d=25',
    '-f',
    'lavfi',
    '-i',
    'sine=frequency=440:duration=25',
    '-c:v',
    'libvpx',
    '-deadline',
    'realtime',
    '-cpu-used',
    '8',
    '-c:a',
    'libopus',
    '-shortest',
    videoFile,
  ])
  ffmpeg([
    '-f',
    'lavfi',
    '-i',
    'sine=frequency=330:duration=25',
    '-ar',
    '8000',
    '-c:a',
    'pcm_s16le',
    audioFile,
  ])
} catch (error) {
  await rm(temporary, { recursive: true, force: true })
  throw error
}
const videoBytes = await readFile(videoFile)
const audioBytes = await readFile(audioFile)
const probe = createServer()
await new Promise((done) => probe.listen(0, '127.0.0.1', done))
const port = probe.address().port
await new Promise((done) => probe.close(done))
const base = `http://127.0.0.1:${port}`
const frontend = spawn(
  process.execPath,
  [
    '-e',
    `
  import { runCommand } from '@nuxt/cli'
  await runCommand('dev', ['--host', '127.0.0.1', '--port', '${port}', '--strictPort', '--no-fork', '--no-tui', '--no-takeover'], {
    overrides: { buildDir: '.data/nuxt-direct-media-fixture', typesDir: '.data/nuxt-direct-media-fixture', nitro: { devStorage: { data: { driver: 'memory' } } } },
  })
`,
  ],
  {
    cwd: frontendDir,
    env: { ...process.env, NUXT_DEMO: 'true', NUXT_BACKEND_URL: 'http://127.0.0.1:1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  },
)
let serverLog = ''
for (const stream of [frontend.stdout, frontend.stderr])
  stream.on('data', (chunk) => {
    serverLog = (serverLog + chunk).slice(-4000)
  })
let browser
let unblockVideo = () => {}
try {
  for (let attempt = 0; ; attempt++) {
    try {
      const app = await fetch(base)
      const entry = await fetch(
        `${base}/_nuxt/@fs${frontendDir}/node_modules/nuxt/dist/app/entry.js`,
      )
      if (app.ok && entry.ok) break
    } catch {
      /* Starting. */
    }
    assert(attempt < 180 && frontend.exitCode === null, `Demo did not start: ${serverLog}`)
    await new Promise((done) => setTimeout(done, 100))
  }
  browser = await chromium.launch({
    headless: true,
    args: ['--autoplay-policy=no-user-gesture-required'],
  })
  const context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `media${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const sourcePage = await (
    await context.request.get(`${base}/api/v1/conversations/general/messages?limit=40`)
  ).json()
  const sample = sourcePage.items.find((item) => item.kind === 'message')
  assert(sample)
  let selected = 'video'
  let holdVideo = true
  let videoRequested
  const videoStarted = new Promise((done) => {
    videoRequested = done
  })
  const videoGate = new Promise((done) => {
    unblockVideo = done
  })
  const attachment = (name, contentType) => ({
    name,
    url: `https://example.invalid/${name}`,
    alt: 'Fixture media',
    contentType,
    width: 160,
    height: 90,
    size: 1024,
  })
  await context.route('**/api/v1/conversations/general/messages?*', (route) => {
    const chosen =
      selected === 'video'
        ? { text: 'Video fixture', attachments: [attachment('fixture.webm', 'video/webm')] }
        : selected === 'audio'
          ? { text: 'Audio fixture', attachments: [attachment('fixture.wav', 'audio/wav')] }
          : { text: 'Card fixture https://example.invalid/post', attachments: [] }
    return route.fulfill({
      json: {
        items: [
          {
            ...sample,
            ...chosen,
            id: `fixture-${selected}`,
            clientId: undefined,
            attachment: undefined,
            reactions: [],
            deleted: false,
            embedGeometry: [],
          },
        ],
        nextCursor: null,
        cursor: 0,
      },
    })
  })
  await context.route('**/link-media', (route) => {
    const url = JSON.parse(route.request().postData() || '{}').url
    return route.fulfill({
      json: {
        url,
        kind: 'card',
        provider: 'page',
        assets: [],
        card: {
          site: 'Example fixture',
          title: 'A local preview without an image',
          description: 'A neutral link card used to check its generated artwork.',
        },
      },
    })
  })
  await context.route('**/example.invalid/**', async (route) => {
    const request = route.request()
    const video = request.url().includes('fixture.webm')
    const bytes = video ? videoBytes : audioBytes
    if (video && holdVideo) {
      videoRequested()
      await videoGate
    }
    const range = request.headers().range?.match(/^bytes=(\d+)-(\d*)$/)
    const start = range ? Number(range[1]) : 0
    const end = range && range[2] ? Math.min(Number(range[2]), bytes.length - 1) : bytes.length - 1
    return route
      .fulfill({
        status: range ? 206 : 200,
        headers: {
          'Content-Type': video ? 'video/webm' : 'audio/wav',
          'Accept-Ranges': 'bytes',
          'Access-Control-Allow-Origin': '*',
          ...(range ? { 'Content-Range': `bytes ${start}-${end}/${bytes.length}` } : {}),
        },
        body: bytes.subarray(start, end + 1),
      })
      .catch(() => {})
  })
  const page = await context.newPage()
  page.setDefaultTimeout(12000)
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('response', (response) => {
    if (response.status() >= 400 && response.url().includes('/nuxt/dist/app/entry.js'))
      console.error('Nuxt entry failed:', response.status(), response.url())
  })
  async function openFixture(kind) {
    selected = kind
    await page.goto(`${base}/channel/general`)
    try {
      await page.waitForFunction(
        () =>
          document
            .querySelector('#__nuxt')
            ?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')?.status === 'connected',
      )
    } catch (error) {
      console.error(
        `Demo did not connect for ${kind}:`,
        page.url(),
        (await page.locator('body').innerText()).slice(0, 1200),
        errors,
        serverLog,
      )
      throw error
    }
    const row = page.locator('.message-row').filter({
      hasText: `${kind === 'video' ? 'Video' : kind === 'audio' ? 'Audio' : 'Card'} fixture`,
    })
    await row.waitFor()
    await row.scrollIntoViewIfNeeded()
    return row
  }
  const near = (actual, expected, label) =>
    assert(Math.abs(actual - expected) < 1, `${label}: ${actual} vs ${expected}`)
  async function bounded(promise, label) {
    let timer
    try {
      return await Promise.race([
        promise,
        new Promise((_, reject) => {
          timer = setTimeout(() => reject(new Error(`${label} timed out`)), 12000)
        }),
      ])
    } finally {
      clearTimeout(timer)
    }
  }
  const mediaState = (media) =>
    media.evaluate((element) => ({
      paused: element.paused,
      muted: element.muted,
      duration: element.duration,
      currentTime: element.currentTime,
    }))
  async function waitPlaying(media, playing) {
    await page.waitForFunction(
      ([selector, expected]) => {
        const element = document.querySelector(selector)
        return element && element.paused === !expected
      },
      [await media.evaluate((element) => element.tagName.toLowerCase()), playing],
    )
  }
  async function checkSeek(group, media, type) {
    const slider = group.getByRole('slider', { name: `Seek ${type}` })
    await slider.focus()
    const duration = (await mediaState(media)).duration
    assert(duration > 20 && Number.isFinite(duration))
    await page.keyboard.press('Home')
    near((await mediaState(media)).currentTime, 0, `${type} Home`)
    await page.keyboard.press('ArrowRight')
    near((await mediaState(media)).currentTime, 5, `${type} ArrowRight`)
    assert.equal(
      Number(await slider.getAttribute('aria-valuenow')),
      5,
      `${type} slider exposes the new time`,
    )
    await page.keyboard.press('ArrowLeft')
    near((await mediaState(media)).currentTime, 0, `${type} ArrowLeft`)
    await page.keyboard.press('End')
    near((await mediaState(media)).currentTime, duration, `${type} End`)
    await page.keyboard.press('ArrowDown')
    near((await mediaState(media)).currentTime, duration - 5, `${type} ArrowDown`)
    await page.keyboard.press('ArrowUp')
    near((await mediaState(media)).currentTime, duration, `${type} ArrowUp`)
    await page.keyboard.press('Home')
  }
  // Reject Satori fonts before the first card renders to exercise its synchronous fallback.
  let fontBlocked
  const blockedFont = new Promise((done) => {
    fontBlocked = done
  })
  const fontPattern = /inter-latin-(400|700)-normal\.woff(?:\?.*)?$/
  await page.route(fontPattern, (route) => {
    if (route.request().resourceType() !== 'fetch') return route.continue()
    fontBlocked()
    return route.abort()
  })
  const fallbackRow = await openFixture('card')
  const fallbackCard = fallbackRow.locator('.link-card')
  await fallbackCard.getByText('A local preview without an image').waitFor()
  await bounded(blockedFont, 'Satori font failure')
  const fallbackArt = await fallbackCard
    .locator('.satori-link-graphic')
    .evaluate((element) => decodeURIComponent(getComputedStyle(element).backgroundImage))
  assert(fallbackArt.includes('<radialGradient') && !fallbackArt.includes('<path'))
  assert.equal(
    await fallbackCard.getByRole('link', { name: /A local preview without an image/ }).count(),
    1,
  )
  await page.screenshot({ path: '../.data/prepush-satori-fallback.png' })
  await page.unroute(fontPattern)
  const videoRow = await openFixture('video')
  const videoGroup = videoRow.getByRole('group', { name: 'Video preview' })
  const video = videoGroup.locator('video')
  await video.waitFor()
  await bounded(videoStarted, 'Video request')
  const beforeVideo = await videoGroup.boundingBox()
  holdVideo = false
  unblockVideo()
  await bounded(
    video.evaluate((element) =>
      element.readyState >= 1
        ? Promise.resolve()
        : new Promise((done) => element.addEventListener('loadedmetadata', done, { once: true })),
    ),
    'Video metadata',
  )
  const afterVideo = await videoGroup.boundingBox()
  near(afterVideo.width, beforeVideo.width, 'video width after metadata')
  near(afterVideo.height, beforeVideo.height, 'video height after metadata')
  await videoGroup.hover()
  await waitPlaying(video, true)
  const videoBar = videoGroup.locator('.video-deck-bar')
  await videoBar.getByRole('button', { name: 'Pause Video' }).waitFor()
  await video.evaluate((element) => {
    window.__fixtureMediaEvents = []
    for (const type of ['play', 'pause', 'volumechange'])
      element.addEventListener(type, () => window.__fixtureMediaEvents.push(type))
  })
  await videoBar.getByRole('button', { name: 'Unmute Video' }).click()
  await page.mouse.move(0, 0)
  await page.waitForTimeout(400)
  assert.deepEqual(
    { paused: (await mediaState(video)).paused, muted: (await mediaState(video)).muted },
    { paused: false, muted: false },
  )
  assert.deepEqual(
    await page.evaluate(() =>
      window.__fixtureMediaEvents.filter((event) => event === 'play' || event === 'pause'),
    ),
    [],
    'video unmute/leave must not flap',
  )
  await videoBar.getByRole('button', { name: 'Mute Video' }).click()
  await page.mouse.move(0, 0)
  await page.waitForTimeout(400)
  assert.deepEqual(
    { paused: (await mediaState(video)).paused, muted: (await mediaState(video)).muted },
    { paused: false, muted: true },
  )
  assert.deepEqual(
    await page.evaluate(() =>
      window.__fixtureMediaEvents.filter((event) => event === 'play' || event === 'pause'),
    ),
    [],
    'video mute/leave must not flap',
  )
  await videoBar.getByRole('button', { name: 'Pause Video' }).click()
  await page.mouse.move(0, 0)
  await waitPlaying(video, false)
  await checkSeek(videoGroup, video, 'Video')
  await videoBar.getByRole('button', { name: 'Play Video' }).click()
  await waitPlaying(video, true)
  await page.waitForTimeout(400)
  assert.equal(
    (await mediaState(video)).paused,
    false,
    'video must continue after seeking and play',
  )
  assert.equal(
    await videoBar.getByRole('button', { name: 'Pause Video' }).count(),
    1,
    'video controls must reflect playback',
  )
  await page.screenshot({ path: '../.data/prepush-direct-video.png' })
  await videoBar.getByRole('button', { name: 'Open full-screen video' }).click()
  await page.locator('.media-viewer').waitFor()
  await waitPlaying(video, false)
  const viewerVideo = page.locator('.media-viewer video')
  await viewerVideo.waitFor()
  await page.waitForFunction(() => document.querySelector('.media-viewer video')?.paused === false)
  await viewerVideo.focus()
  await page.keyboard.press('Space')
  await page.waitForFunction(() => document.querySelector('.media-viewer video')?.paused === true)
  await page.screenshot({ path: '../.data/prepush-direct-viewer.png' })
  await page.getByRole('button', { name: 'Close viewer' }).click()
  await page.locator('.media-viewer').waitFor({ state: 'hidden' })
  assert.equal((await mediaState(video)).paused, true)
  near(
    (await videoGroup.boundingBox()).height,
    beforeVideo.height,
    'video height after controls and viewer',
  )

  const audioRow = await openFixture('audio')
  const audioGroup = audioRow.getByRole('group', { name: 'Audio preview' })
  const audio = audioGroup.locator('audio')
  await audio.waitFor({ state: 'attached' })
  const beforeAudio = await audioGroup.boundingBox()
  await bounded(
    audio.evaluate((element) =>
      element.readyState >= 1
        ? Promise.resolve()
        : new Promise((done) => element.addEventListener('loadedmetadata', done, { once: true })),
    ),
    'Audio metadata',
  )
  near((await audioGroup.boundingBox()).height, beforeAudio.height, 'audio height after metadata')
  await audioGroup.getByRole('button', { name: 'Play Audio' }).click()
  await waitPlaying(audio, true)
  await audioGroup.getByRole('button', { name: 'Pause Audio' }).waitFor()
  await audio.evaluate((element) => {
    window.__fixtureMediaEvents = []
    for (const type of ['play', 'pause', 'volumechange'])
      element.addEventListener(type, () => window.__fixtureMediaEvents.push(type))
  })
  await audioGroup.getByRole('button', { name: 'Mute Audio' }).click()
  await page.mouse.move(0, 0)
  await page.waitForTimeout(400)
  assert.deepEqual(
    { paused: (await mediaState(audio)).paused, muted: (await mediaState(audio)).muted },
    { paused: false, muted: true },
  )
  assert.deepEqual(
    await page.evaluate(() =>
      window.__fixtureMediaEvents.filter((event) => event === 'play' || event === 'pause'),
    ),
    [],
    'audio mute/leave must not flap',
  )
  await audioGroup.getByRole('button', { name: 'Unmute Audio' }).click()
  await page.mouse.move(0, 0)
  await page.waitForTimeout(400)
  assert.deepEqual(
    { paused: (await mediaState(audio)).paused, muted: (await mediaState(audio)).muted },
    { paused: false, muted: false },
  )
  assert.deepEqual(
    await page.evaluate(() =>
      window.__fixtureMediaEvents.filter((event) => event === 'play' || event === 'pause'),
    ),
    [],
    'audio unmute/leave must not flap',
  )
  await audioGroup.getByRole('button', { name: 'Pause Audio' }).click()
  await waitPlaying(audio, false)
  await checkSeek(audioGroup, audio, 'Audio')
  near((await audioGroup.boundingBox()).height, beforeAudio.height, 'audio height after controls')
  await page.screenshot({ path: '../.data/prepush-direct-audio.png' })

  const cardRow = await openFixture('card')
  const card = cardRow.locator('.link-card')
  await card.getByText('A local preview without an image').waitFor()
  near((await card.boundingBox()).height, 150, 'fallback link card height')
  const graphic = card.locator('.satori-link-graphic')
  await graphic.waitFor()
  await page.waitForFunction(() =>
    getComputedStyle(document.querySelector('.satori-link-graphic')).backgroundImage.includes(
      'data:image/svg+xml',
    ),
  )
  const art = await graphic.evaluate((element) =>
    decodeURIComponent(getComputedStyle(element).backgroundImage),
  )
  assert(art.includes('<svg') && art.includes('</svg>'))
  assert(art.includes('<path'), 'The Satori poster should render when its fonts load')
  assert.equal(
    await card.getByRole('link', { name: /A local preview without an image/ }).count(),
    1,
  )
  const cardDescription = await card.locator('.link-card-description').evaluate((element) => ({
    text: element.textContent,
    display: getComputedStyle(element).display,
  }))
  assert.equal(cardDescription.text, 'A neutral link card used to check its generated artwork.')
  assert.notEqual(cardDescription.display, 'none', 'Normal cards retain their description after borders')
  assert(await card.locator('.link-card-description').isVisible())
  const cardContentHeight = await card.evaluate((element) => element.clientHeight)
  console.log(
    `Card description at 150px outer / ${cardContentHeight}px inner: ${cardDescription.display}`,
  )
  await page.screenshot({ path: '../.data/prepush-satori-card.png' })

  await page.setViewportSize({ width: 390, height: 844 })
  for (const kind of ['video', 'audio']) {
    const row = await openFixture(kind)
    const group = row.getByRole('group', {
      name: `${kind === 'video' ? 'Video' : 'Audio'} preview`,
    })
    await group.waitFor()
    const bounds = await group.boundingBox()
    assert(
      bounds.width <= 390 && bounds.x >= 0 && bounds.x + bounds.width <= 390.5,
      `${kind} exceeds phone viewport`,
    )
    const slider = group.getByRole('slider', {
      name: `Seek ${kind === 'video' ? 'Video' : 'Audio'}`,
    })
    const sliderBox = await slider.boundingBox()
    assert(
      sliderBox.width >= 24 &&
        sliderBox.x >= bounds.x &&
        sliderBox.x + sliderBox.width <= bounds.x + bounds.width,
      `${kind} slider does not fit`,
    )
    for (const button of await group.locator('button').all()) {
      const rect = await button.boundingBox()
      if (rect)
        assert(
          rect.x >= bounds.x - 0.5 && rect.x + rect.width <= bounds.x + bounds.width + 0.5,
          `${kind} button overflows`,
        )
    }
    await page.screenshot({ path: `../.data/prepush-direct-${kind}-phone.png` })
  }
  assert.deepEqual(errors, [])
  console.log(
    `PASS: video/audio controls, hover ownership, keyboard seeking, geometry, viewer and phone; link art ${art.includes('<path') ? 'Satori-rendered' : 'fallback SVG'}`,
  )
} finally {
  unblockVideo()
  await browser?.close()
  frontend.kill('SIGTERM')
  await rm(temporary, { recursive: true, force: true })
}
