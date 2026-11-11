// Run against an isolated NUXT_DEMO server. All accounts and links are disposable fixtures.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium } from 'playwright'
import { sanitizePreferences } from '../shared/preferences.ts'

const base = process.env.NSFW_TEST_BASE || 'http://127.0.0.1:4178'
const output = process.env.NSFW_TEST_OUTPUT || '../.data'
assert.deepEqual(
  sanitizePreferences({ nsfwNetworks: { libera: true, ergo: false, 'bad/key': true } })
    .nsfwNetworks,
  { libera: true },
)
const browser = await chromium.launch({ headless: true })
const errors = []
const source = (conversationId, networkId) => ({
  conversationId,
  networkId,
  nsfw: true,
  nsfwKnown: true,
  messageId: `fixture-${networkId}`,
  senderId: 'you',
  postedAt: '2026-01-01T12:00:00Z',
  url: 'https://example.invalid/neutral',
  excerpt: 'neutral fixture',
})
const card = (identity, sources) => ({
  identity,
  url: `https://example.invalid/${identity}`,
  host: 'example.invalid',
  kind: 'link',
  count: sources.length,
  lastPostedAt: '2026-01-01T12:00:00Z',
  sources,
})
const cards = [
  card('first', [
    source('general', 'libera'),
    { ...source('general', 'libera'), messageId: 'fixture-repost' },
  ]),
  card('second', [source('ergo-lounge', 'ergo')]),
  card('mixed', [source('general', 'libera'), source('ergo-lounge', 'ergo')]),
]
let requests = []
async function fixture() {
  const context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: {
      username: `nsfw${randomUUID().replaceAll('-', '').slice(0, 14)}`,
      password: randomUUID(),
    },
  })
  assert.equal(signup.status(), 201)
  const updated = await context.request.put(
    `${base}/api/v1/conversations/general/channel-settings`,
    { data: { nsfw: true } },
  )
  assert.equal(updated.status(), 200)
  const actualBootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  const actualLinks = await (await context.request.get(`${base}/api/v1/links`)).json()
  const actualSources = actualLinks.items.flatMap((item) => item.sources)
  assert(actualSources.length > 0, 'demo contains link sources')
  for (const item of actualSources) {
    const channel = actualBootstrap.conversations.find(
      (c) => c.id === item.conversationId && c.networkId === item.networkId,
    )
    assert.equal(item.nsfwKnown, true)
    assert.equal(item.nsfw, channel.channelSettings.nsfw)
  }
  await context.route('**/api/v1/bootstrap', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    const channel = body.conversations.find((c) => c.id === 'ergo-lounge')
    channel.channelSettings.nsfw = true
    await route.fulfill({ response, json: body })
  })
  await context.route('**/api/v1/links?*', (route) =>
    route.fulfill({ json: { items: cards, nextCursor: null, indexing: false } }),
  )
  await context.route('**/link-media', (route) => {
    requests.push(route.request().url())
    const url = JSON.parse(route.request().postData() || '{}').url
    if (url === 'https://example.invalid/first')
      return route.fulfill({
        json: {
          url,
          kind: 'card',
          provider: 'page',
          assets: [],
          card: {
            title: 'A neutral fixture with a long title for a rich catalog card',
            description:
              'A longer neutral description that exercises the reserved body, source caption, excerpt, and repeated-post menu.',
          },
        },
      })
    return route.abort()
  })
  await context.route('**/example.invalid/**', (route) => {
    requests.push(route.request().url())
    return route.abort()
  })
  const page = await context.newPage()
  page.on('pageerror', (error) => errors.push(error.message))
  page.setDefaultTimeout(12000)
  await page.goto(`${base}/links`)
  await page.locator('.catalog-card').first().waitFor()
  return { context, page }
}
async function profile(page) {
  await page.getByRole('button', { name: 'User settings' }).click()
  const dialog = page.getByRole('dialog', { name: 'Settings' })
  await dialog.locator('[data-section="profile"]').click()
  await dialog.locator('#profile-server').waitFor()
  return dialog
}

// Mock only the private event stream and catalog: the two tabs share a real disposable
// demo account and its preferences document. Responses use the generated wire shapes.
async function protocolFixture() {
  const context = await browser.newContext({ serviceWorkers: 'block' })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: {
      username: `race${randomUUID().replaceAll('-', '').slice(0, 14)}`,
      password: randomUUID(),
    },
  })
  assert.equal(signup.status(), 201)
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  const retained = bootstrap.conversations.find((c) => c.id === 'general')
  retained.joined = false
  delete retained.channelSettings
  bootstrap.cursor = 0
  const socketByPage = []
  let connectedResolve
  const connected = new Promise((resolve) => {
    connectedResolve = resolve
  })
  await context.routeWebSocket('**/api/v1/events*', (socket) => {
    socketByPage.push(socket)
    socket.onMessage((message) => {
      const event = JSON.parse(String(message))
      if (event.type === 'session.resume') {
        socket.send(JSON.stringify({ v: 1, type: 'session.synced', cursor: 0 }))
        connectedResolve()
      }
    })
    socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: 0, sync: true }))
  })
  await context.route('**/api/v1/bootstrap', (route) => route.fulfill({ json: bootstrap }))
  const safe = card('retained', [{ ...source('general', 'libera'), nsfw: false }])
  const sensitive = card('retained', [{ ...source('general', 'libera'), nsfw: true }])
  let phase = 'safe'
  let held = []
  let requestedResolve
  const requested = new Promise((resolve) => {
    requestedResolve = resolve
  })
  const previewRequests = []
  await context.route('**/api/v1/links?*', async (route) => {
    if (phase === 'holding') {
      held.push(route)
      requestedResolve()
      return
    }
    await route.fulfill({
      json: { items: [phase === 'safe' ? safe : sensitive], nextCursor: null, indexing: false },
    })
  })
  await context.route('**/link-media', (route) => {
    previewRequests.push(route.request().url())
    return route.abort()
  })
  await context.route('**/example.invalid/**', (route) => {
    previewRequests.push(route.request().url())
    return route.abort()
  })
  const first = await context.newPage()
  first.setDefaultTimeout(12000)
  first.on('pageerror', (error) => errors.push(error.message))
  await first.goto(`${base}/links`)
  await first.locator('.catalog-card-title').waitFor()
  await Promise.race([
    connected,
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error('Private socket did not connect')), 12000),
    ),
  ])
  assert.equal(socketByPage.length, 1)
  const send = (sequence, type, data) =>
    socketByPage[0].send(JSON.stringify({ v: 1, seq: sequence, type, data }))
  phase = 'holding'
  send(1, 'conversation.updated', retained)
  await Promise.race([
    requested,
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error('Retained update did not refetch Links')), 12000),
    ),
  ])
  await first.getByText('Channel settings unavailable', { exact: true }).first().waitFor()
  assert.equal(await first.locator('.catalog-card-title').count(), 0)
  const requestsAtGate = previewRequests.length
  phase = 'sensitive'
  send(2, 'conversation.updated', retained)
  await first.getByText('NSFW link', { exact: true }).first().waitFor()
  // Release the older false classification after the newer true response has landed.
  for (const route of held)
    await route
      .fulfill({ json: { items: [safe], nextCursor: null, indexing: false } })
      .catch(() => {})
  console.log('retained race exercised')
  await first.waitForTimeout(500)
  assert.equal(
    await first.locator('.catalog-card-title').count(),
    0,
    'old response must not reveal the card',
  )
  assert.equal(
    previewRequests.length,
    requestsAtGate,
    'no previews start while classification is gated',
  )
  await first.getByRole('button', { name: 'Show link' }).click()
  await first.locator('.catalog-card-title').waitFor()
  sensitive.sources[0].messageId = 'fixture-replacement'
  send(3, 'conversation.updated', retained)
  await first.getByText('NSFW link', { exact: true }).first().waitFor()
  assert.equal(
    await first.locator('.catalog-card-title').count(),
    0,
    'a new source identity must reset reveal',
  )

  const other = await context.newPage()
  other.setDefaultTimeout(12000)
  other.on('pageerror', (error) => errors.push(error.message))
  await other.goto(`${base}/links`)
  await other.getByText('NSFW link', { exact: true }).first().waitFor()
  let dialog = await profile(other)
  await dialog.locator('#profile-server').selectOption('libera')
  const toggle = dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' })
  const save = () =>
    other.waitForResponse(
      (response) =>
        response.url().endsWith('/api/app/preferences') &&
        response.request().method() === 'PUT' &&
        response.ok(),
    )
  await Promise.all([save(), toggle.click()])
  send(4, 'client-preferences.updated', { revision: 'fixture-4' })
  await first.locator('.catalog-card-title').waitFor()
  await Promise.all([save(), toggle.click()])
  let releaseRead
  let readStarted
  let readFinished
  const readSeen = new Promise((resolve) => {
    readStarted = resolve
  })
  const readRelease = new Promise((resolve) => {
    releaseRead = resolve
  })
  const readComplete = new Promise((resolve) => {
    readFinished = resolve
  })
  await first.route('**/api/app/preferences', async (route) => {
    if (route.request().method() !== 'GET') return route.continue()
    const response = await route.fetch()
    readStarted()
    await readRelease
    await route.fulfill({ response }).catch(() => {})
    readFinished()
  })
  send(5, 'client-preferences.updated', { revision: 'fixture-5' })
  await readSeen
  await first.getByText('NSFW link', { exact: true }).first().waitFor()
  assert.equal(await first.locator('.catalog-card-title').count(), 0)
  releaseRead()
  await readComplete
  console.log('cross-tab refresh exercised')
  await first.unroute('**/api/app/preferences')

  // A failed local opt-in must not mask a later private update indefinitely.
  await first.route('**/api/app/preferences', (route) =>
    route.request().method() === 'PUT'
      ? route.fulfill({ status: 503, json: { message: 'fixture failure' } })
      : route.continue(),
  )
  dialog = await profile(first)
  await dialog.locator('#profile-server').selectOption('libera')
  const failed = first.waitForResponse(
    (response) =>
      response.url().endsWith('/api/app/preferences') &&
      response.request().method() === 'PUT' &&
      response.status() === 503,
  )
  await dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' }).click()
  await failed
  await first.keyboard.press('Escape')
  send(6, 'client-preferences.updated', { revision: 'fixture-6' })
  await first.getByText('NSFW link', { exact: true }).first().waitFor()
  assert.equal(
    await first.locator('.catalog-card-title').count(),
    0,
    'failed local save must not hide a later private update',
  )
  await first.getByRole('button', { name: 'Show link' }).click()
  await first.locator('.catalog-card-title').waitFor()

  // An older successful save response arrives after a different tab has opted out.
  await first.unroute('**/api/app/preferences')
  let sentResolve
  const sent = new Promise((resolve) => {
    sentResolve = resolve
  })
  let releaseResolve
  const release = new Promise((resolve) => {
    releaseResolve = resolve
  })
  await first.route('**/api/app/preferences', async (route) => {
    if (route.request().method() !== 'PUT') return route.continue()
    const response = await route.fetch()
    sentResolve()
    await release
    await route.fulfill({ response })
  })
  dialog = await profile(first)
  await dialog.locator('#profile-server').selectOption('libera')
  const firstToggle = dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' })
  await firstToggle.click()
  await sent
  await Promise.all([save(), toggle.click()])
  await Promise.all([save(), toggle.click()])
  send(7, 'client-preferences.updated', { revision: 'fixture-7' })
  await first.getByText('NSFW link', { exact: true }).first().waitFor()
  assert.equal(
    await first.locator('.catalog-card-title').count(),
    0,
    'newer private opt-out must take effect during an older save',
  )
  releaseResolve()
  await first.waitForTimeout(700)
  assert.equal(
    await first.locator('.catalog-card-title').count(),
    0,
    'older save response must not override a private opt-out',
  )
  console.log('cross-tab save race exercised')
  await context.close()
}

async function timelineFixture() {
  const context = await browser.newContext({
    serviceWorkers: 'block',
    hasTouch: !process.env.NSFW_FINE_POINTER,
    viewport: process.env.NSFW_MOBILE_GEOMETRY
      ? { width: 390, height: 844 }
      : { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: {
      username: `media${randomUUID().replaceAll('-', '').slice(0, 14)}`,
      password: randomUUID(),
    },
  })
  assert.equal(signup.status(), 201)
  const updated = await context.request.put(
    `${base}/api/v1/conversations/general/channel-settings`,
    { data: { nsfw: true } },
  )
  assert.equal(updated.status(), 200)
  const sourcePage = await (
    await context.request.get(`${base}/api/v1/conversations/general/messages?limit=40`)
  ).json()
  const sample = sourcePage.items.find((item) => item.kind === 'message')
  assert(sample)
  const upload = (name, contentType = 'image/png') => ({
    name,
    url: `https://example.invalid/${name}`,
    alt: 'Neutral fixture',
    contentType,
    width: 100,
    height: 100,
    size: 64,
  })
  const cases = [
    {
      id: 'fixture-image',
      text: 'Image fixture https://example.invalid/image.png',
      media: '.message-link',
    },
    {
      id: 'fixture-gif',
      text: 'GIF fixture https://example.invalid/animation.gif',
      media: '.message-link',
    },
    {
      id: 'fixture-widget',
      text: 'Widget fixture https://dbdiagram.io/d/fixture',
      media: '.message-link',
    },
    {
      id: 'fixture-video',
      text: 'Video fixture',
      attachments: [upload('neutral.mp4', 'video/mp4')],
      media: '.message-attachment',
    },
    {
      id: 'fixture-audio',
      text: 'Audio fixture',
      attachments: [upload('neutral.mp3', 'audio/mpeg')],
      media: '.message-attachment',
    },
    {
      id: 'fixture-gallery',
      text: 'Gallery fixture',
      attachments: [upload('one.png'), upload('two.png')],
      media: '.message-attachment-gallery',
    },
  ]
  let selected = cases[0]
  let holdLink = false
  let linkStarted
  const linkRequested = new Promise((resolve) => {
    linkStarted = resolve
  })
  let releaseLink
  const linkRelease = new Promise((resolve) => {
    releaseLink = resolve
  })
  const requests = []
  const png = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=',
    'base64',
  )
  await context.route('**/api/v1/conversations/general/messages?*', (route) => {
    const message = {
      ...sample,
      id: selected.id,
      clientId: undefined,
      text: selected.text,
      attachments: selected.attachments,
      attachment: undefined,
      reactions: [],
      deleted: false,
      embedGeometry: [],
    }
    return route.fulfill({ json: { items: [message], nextCursor: null, cursor: 0 } })
  })
  await context.route('**/link-media', async (route) => {
    requests.push(route.request().url())
    const url = JSON.parse(route.request().postData() || '{}').url
    const widget = url?.includes('dbdiagram.io')
    if (holdLink) {
      linkStarted()
      await linkRelease
    }
    return route
      .fulfill({
        json: widget
          ? {
              url,
              kind: 'widget',
              provider: 'oembed',
              assets: [],
              title: 'Neutral fixture',
              widget: {
                url: 'https://dbdiagram.io/embed/fixture',
                external: true,
                width: 640,
                height: 360,
                token: 'a'.repeat(48),
              },
            }
          : {
              url,
              kind: 'media',
              provider: 'direct',
              assets: [
                {
                  url: 'https://example.invalid/neutral.png',
                  kind: 'image',
                  contentType: url?.endsWith('.gif') ? 'image/gif' : 'image/png',
                  width: 100,
                  height: 100,
                },
              ],
            },
      })
      .catch(() => {})
  })
  await context.route('**/example.invalid/**', (route) => {
    requests.push(route.request().url())
    return route.fulfill({ status: 200, contentType: 'image/png', body: png })
  })
  await context.route('**/dbdiagram.io/**', (route) => {
    requests.push(route.request().url())
    return route.fulfill({ status: 200, contentType: 'text/html', body: '<p>Neutral fixture</p>' })
  })
  const page = await context.newPage()
  page.setDefaultTimeout(12000)
  page.on('pageerror', (error) => errors.push(error.message))
  const save = () =>
    page.waitForResponse(
      (response) =>
        response.url().endsWith('/api/app/preferences') &&
        response.request().method() === 'PUT' &&
        response.ok(),
    )
  const geometryOnly = !!process.env.NSFW_UPLOAD_GEOMETRY_ONLY
  for (const item of geometryOnly
    ? cases.filter((item) => ['fixture-video', 'fixture-audio'].includes(item.id))
    : cases) {
    console.log('timeline fixture', item.id)
    selected = item
    requests.length = 0
    await page.goto(`${base}/channel/general`)
    const row = page
      .locator('.message-row')
      .filter({ hasText: item.text.split(' ')[0] + ' fixture' })
    await row.waitFor()
    const box = row.locator(item.media)
    await box.locator('.sensitive-media').waitFor()
    await box.scrollIntoViewIfNeeded()
    await page.waitForTimeout(250)
    assert.equal(requests.length, 0, `${item.id} fetched while blocked`)
    assert.equal(await box.locator('img, video, audio, iframe').count(), 0)
    if (item.id === 'fixture-video' || item.id === 'fixture-audio') {
      assert.equal(await box.locator('a').count(), 0)
      assert.equal((await box.innerText()).includes(item.attachments[0].name), false)
    }
    const before = await box.boundingBox()
    const rowBefore = await row.boundingBox()
    if (item.id === 'fixture-video' || item.id === 'fixture-audio') {
      await box.getByRole('button', { name: 'Show media' }).click()
      await box.locator('.message-file').waitFor()
      const manual = await box.boundingBox()
      const manualRow = await row.boundingBox()
      assert(
        Math.abs(before.height - manual.height) < 1 &&
          Math.abs(rowBefore.height - manualRow.height) < 1,
        `${item.id} resized after manual reveal`,
      )
      assert.equal(
        await box.locator('.message-file').getAttribute('download'),
        item.attachments[0].name,
      )
      await page.waitForFunction(
        (name) =>
          document.querySelector('.message-file')?.getAttribute('href') ===
          `https://example.invalid/${name}`,
        item.attachments[0].name,
      )
    }
    if (item.id === 'fixture-gallery') {
      await box.getByRole('button', { name: 'Show media' }).click()
      await box.locator('.media-gallery-mosaic').waitFor()
      const revealedBox = await box.boundingBox()
      assert(
        Math.abs(before.width - revealedBox.width) < 1 &&
          Math.abs(before.height - revealedBox.height) < 1,
      )
      const stored = await (await context.request.get(`${base}/api/app/preferences`)).json()
      assert.notEqual(stored.preferences?.nsfwNetworks?.libera, true)
      await page.reload()
      await box.locator('.sensitive-media').waitFor()
    }
    if (item.id === 'fixture-widget') {
      const stored = await (await context.request.get(`${base}/api/app/preferences`)).json()
      stored.preferences.askBeforeEmbeds = true
      assert.equal(
        (
          await context.request.put(`${base}/api/app/preferences`, {
            data: { preferences: stored.preferences },
          })
        ).status(),
        200,
      )
      await page.reload()
      await box.getByRole('button', { name: 'Show media' }).click()
      await box.locator('.widget-poster').waitFor()
      assert.equal(
        await box.locator('iframe').count(),
        0,
        'manual reveal must respect embed consent',
      )
      stored.preferences.askBeforeEmbeds = false
      assert.equal(
        (
          await context.request.put(`${base}/api/app/preferences`, {
            data: { preferences: stored.preferences },
          })
        ).status(),
        200,
      )
      await page.reload()
      await box.locator('.sensitive-media').waitFor()
    }
    let dialog = await profile(page)
    if (item.id === 'fixture-image') {
      await dialog.locator('#profile-server').selectOption('ergo')
      await Promise.all([
        save(),
        dialog.getByRole('switch', { name: 'Show NSFW content on Ergo' }).click(),
      ])
      assert.equal(
        await box.locator('.sensitive-media').count(),
        1,
        'another server opt-in must not reveal this message',
      )
    }
    await dialog.locator('#profile-server').selectOption('libera')
    await Promise.all([
      save(),
      dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' }).click(),
    ])
    await page.keyboard.press('Escape')
    await box.locator('.sensitive-media').waitFor({ state: 'detached' })
    await box.scrollIntoViewIfNeeded()
    const after = await box.boundingBox()
    assert(
      Math.abs(before.width - after.width) < 1 && Math.abs(before.height - after.height) < 1,
      `${item.id} resized after opt-in`,
    )
    const rowAfter = await row.boundingBox()
    assert(
      Math.abs(rowBefore.height - rowAfter.height) < 1,
      `${item.id} message row resized after opt-in`,
    )
    if (item.id === 'fixture-video' || item.id === 'fixture-audio')
      await box.locator('.message-file').waitFor()
    if (item.id === 'fixture-image' || item.id === 'fixture-gif')
      await box.locator('.direct-media-frame').waitFor()
    if (item.id === 'fixture-gallery') await box.locator('.media-gallery-mosaic').waitFor()
    if (item.id === 'fixture-widget') {
      await box
        .locator('iframe')
        .waitFor()
        .catch(async (error) => {
          console.log('Widget diagnostics', await box.innerText(), requests)
          throw error
        })
    }
    if (item.id === 'fixture-image') {
      await box.locator('.direct-media-frame').click()
      await page.locator('.media-viewer').waitFor()
    }
    if (item.id === 'fixture-image') {
      await Promise.all([
        save(),
        page.evaluate(() => {
          const preferences = document
            .querySelector('#__nuxt')
            .__vue_app__.config.globalProperties.$pinia._s.get('preferences')
          const nsfwNetworks = { ...preferences.preferences.nsfwNetworks }
          delete nsfwNetworks.libera
          preferences.update({ nsfwNetworks })
        }),
      ])
    } else {
      dialog = await profile(page)
      await dialog.locator('#profile-server').selectOption('libera')
      await Promise.all([
        save(),
        dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' }).click(),
      ])
      await page.keyboard.press('Escape')
    }
    await box.locator('.sensitive-media').waitFor()
    assert.equal(await page.locator('.media-viewer').count(), 0)
    assert.equal(await box.locator('img, video, audio, iframe').count(), 0)
    if (item.id === 'fixture-video' || item.id === 'fixture-audio')
      assert.equal(await box.locator('a').count(), 0)
    const blockedAgain = await box.boundingBox()
    assert(
      Math.abs(before.width - blockedAgain.width) < 1 &&
        Math.abs(before.height - blockedAgain.height) < 1,
      `${item.id} resized after opt-out`,
    )
    const rowBlockedAgain = await row.boundingBox()
    assert(
      Math.abs(rowBefore.height - rowBlockedAgain.height) < 1,
      `${item.id} message row resized after opt-out`,
    )
  }
  if (geometryOnly) {
    await context.close()
    console.log('PASS: video and audio upload full row geometry, hidden footer, and download link')
    return
  }
  const gallery = page.locator('.message-attachment-gallery')
  await page.screenshot({ path: `${output}/nsfw-timeline-desktop.png` })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: `${output}/nsfw-timeline-mobile.png` })
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.getByText('Connected', { exact: true }).first().waitFor()
  assert.equal(
    (
      await context.request.put(`${base}/api/v1/conversations/general/channel-settings`, {
        data: { nsfw: false },
      })
    ).status(),
    200,
  )
  await gallery.locator('.media-gallery-tile').first().waitFor()
  await gallery.locator('.media-gallery-tile').first().click()
  await page.locator('.media-viewer').waitFor()
  assert.equal(
    (
      await context.request.put(`${base}/api/v1/conversations/general/channel-settings`, {
        data: { nsfw: true },
      })
    ).status(),
    200,
  )
  await gallery.locator('.sensitive-media').waitFor()
  assert.equal(
    await page.locator('.media-viewer').count(),
    0,
    'reclassification closes the open viewer',
  )
  assert.equal(await gallery.locator('img, video, audio, iframe').count(), 0)
  selected = {
    id: 'fixture-late',
    text: 'Late fixture https://example.invalid/late.png',
    media: '.message-link',
  }
  holdLink = true
  await page.goto(`${base}/channel/general`)
  const late = page
    .locator('.message-row')
    .filter({ hasText: 'Late fixture' })
    .locator('.message-link')
  await late.locator('.sensitive-media').waitFor()
  const lateDialog = await profile(page)
  await lateDialog.locator('#profile-server').selectOption('libera')
  await Promise.all([
    save(),
    lateDialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' }).click(),
  ])
  await Promise.race([
    linkRequested,
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error('Preview request did not start')), 12000),
    ),
  ])
  await Promise.all([
    save(),
    lateDialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' }).click(),
  ])
  await late.locator('.sensitive-media').waitFor()
  releaseLink()
  await page.waitForTimeout(500)
  assert.equal(
    await late.locator('img, video, audio, iframe').count(),
    0,
    'a late preview response must stay hidden',
  )
  holdLink = false
  selected = {
    id: 'fixture-pair',
    text: 'Pair fixture https://example.invalid/a.png https://example.invalid/b.gif',
  }
  await page.goto(`${base}/channel/general`)
  const pair = page.locator('.message-row').filter({ hasText: 'Pair fixture' })
  await pair.locator('.sensitive-media').nth(1).waitFor()
  await pair.getByRole('button', { name: 'Show media' }).first().focus()
  await page.keyboard.press('Enter')
  await pair.locator('.sensitive-media').nth(1).waitFor({ state: 'detached' })
  assert.equal(
    await pair.locator('.sensitive-media').count(),
    1,
    'keyboard reveal affects one preview',
  )
  assert.notEqual(
    (await (await context.request.get(`${base}/api/app/preferences`)).json()).preferences
      ?.nsfwNetworks?.libera,
    true,
  )
  await pair.getByRole('button', { name: 'Show media' }).tap()
  assert.equal(await pair.locator('.sensitive-media').count(), 0)
  const pairDialog = await profile(page)
  await pairDialog.locator('#profile-server').selectOption('libera')
  const pairSwitch = pairDialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' })
  await Promise.all([save(), pairSwitch.click()])
  await Promise.all([save(), pairSwitch.click()])
  await page.keyboard.press('Escape')
  await pair.locator('.sensitive-media').nth(1).waitFor()
  await page.reload()
  await pair.locator('.sensitive-media').nth(1).waitFor()
  selected = {
    id: 'fixture-file',
    text: 'File fixture',
    attachments: [upload('neutral.txt', 'text/plain')],
  }
  await page.goto(`${base}/channel/general`)
  const file = page
    .locator('.message-row')
    .filter({ hasText: 'File fixture' })
    .locator('.message-file')
  await file.waitFor()
  assert.equal(await file.getAttribute('href'), 'https://example.invalid/neutral.txt')
  assert.equal(await file.getAttribute('download'), 'neutral.txt')
  await context.close()
  console.log(
    'PASS: timeline direct image, GIF, widget and upload gallery gating, geometry, server isolation and viewer dismissal on opt-out and reclassification',
  )
}
if (process.env.NSFW_TIMELINE_ONLY) {
  try {
    await timelineFixture()
    assert.deepEqual(errors, [])
  } finally {
    await browser.close()
  }
  process.exit(0)
}
try {
  if (!process.env.NSFW_CATALOG_ONLY) {
    await protocolFixture()
    await timelineFixture()
  }
  const first = await fixture()
  const { page } = first
  await page.waitForTimeout(500)
  assert.equal(await page.locator('.catalog-card-nsfw').count(), 3)
  assert.equal(await page.locator('.catalog-card a').count(), 0)
  assert.equal(requests.length, 0, 'gated cards must not request previews or images')
  await page.screenshot({ path: `${output}/nsfw-links-desktop.png` })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: `${output}/nsfw-links-mobile.png` })
  await page.setViewportSize({ width: 1440, height: 900 })
  const hiddenCards = page.locator('.catalog-card-nsfw')
  const wholeCardBefore = await hiddenCards.first().boundingBox()
  const firstCardBefore = await hiddenCards.first().locator('.aspect-video').boundingBox()
  await hiddenCards.first().getByRole('button', { name: 'Show link' }).click()
  assert.equal(await page.locator('.catalog-card-nsfw').count(), 2)
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/first"]').count(),
    1,
  )
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/second"]').count(),
    0,
  )
  const firstCardAfter = await page.locator('.catalog-card-preview').first().boundingBox()
  await page
    .locator('.catalog-card-title')
    .filter({ hasText: 'A neutral fixture with a long title for a rich catalog card' })
    .waitFor()
  assert.equal(await page.locator('.catalog-card-count').first().isVisible(), true)
  assert.equal(await page.locator('.catalog-card-description').first().isVisible(), true)
  await page.screenshot({ path: `${output}/nsfw-links-rich-revealed.png` })
  const wholeCardAfter = await page.locator('.catalog-card').first().boundingBox()
  assert(
    Math.abs(wholeCardBefore.width - wholeCardAfter.width) < 1 &&
      Math.abs(wholeCardBefore.height - wholeCardAfter.height) < 1,
    'rich catalog card resized after reveal',
  )
  assert(
    Math.abs(firstCardBefore.width - firstCardAfter.width) < 1 &&
      Math.abs(firstCardBefore.height - firstCardAfter.height) < 1,
  )
  assert.deepEqual(
    (await (await first.context.request.get(`${base}/api/app/preferences`)).json()).preferences
      ?.nsfwNetworks ?? {},
    {},
  )
  await page.reload()
  await page.locator('.catalog-card-nsfw').nth(2).waitFor()
  await page
    .locator('.catalog-card-nsfw')
    .first()
    .getByRole('button', { name: 'Show link' })
    .focus()
  await page.keyboard.press('Space')
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 2)
  await page.reload()
  await page.locator('.catalog-card-nsfw').nth(2).waitFor()
  await page.getByRole('textbox', { name: 'Filter links' }).focus()
  await page.keyboard.press('Enter')
  await page.waitForURL(`${base}/channel/general`)
  await page.goto(`${base}/links`)
  await page.locator('.catalog-card-nsfw').first().waitFor()
  let dialog = await profile(page)
  await dialog.locator('#profile-server').selectOption('libera')
  const libera = dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' })
  assert.equal(await libera.getAttribute('aria-checked'), 'false')
  await Promise.all([
    page.waitForResponse(
      (response) =>
        response.url().endsWith('/api/app/preferences') &&
        response.request().method() === 'PUT' &&
        response.ok(),
    ),
    libera.click(),
  ])
  await page.keyboard.press('Escape')
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 2)
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/first"]').count(),
    1,
  )
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/second"]').count(),
    0,
  )
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/mixed"]').count(),
    0,
  )
  await page
    .locator('.catalog-card')
    .filter({ has: page.locator('.catalog-card-title[href="https://example.invalid/first"]') })
    .locator('.catalog-card-source')
    .click()
  await page.waitForURL(`${base}/channel/general`)
  await page.goto(`${base}/links`)
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 2)
  const second = await fixture()
  assert.equal(
    await second.page.locator('.catalog-card-nsfw').count(),
    3,
    'another account defaults to off',
  )
  dialog = await profile(page)
  await dialog.locator('#profile-server').selectOption('libera')
  await first.context.route('**/api/app/preferences', async (route) => {
    if (route.request().method() !== 'PUT') return route.continue()
    const response = await route.fetch()
    await new Promise((resolve) => setTimeout(resolve, 700))
    await route.fulfill({ response })
  })
  const preferenceSaved = page.waitForResponse(
    (response) =>
      response.url().endsWith('/api/app/preferences') &&
      response.request().method() === 'PUT' &&
      response.ok(),
  )
  await dialog.getByRole('switch', { name: 'Show NSFW content on Libera Chat' }).click()
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 3)
  assert.equal(await page.locator('.catalog-card a').count(), 0)
  await preferenceSaved
  await page.keyboard.press('Escape')
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 3)
  assert.equal(await page.locator('.catalog-card a').count(), 0)
  const wholeCardBlockedAgain = await page.locator('.catalog-card-nsfw').first().boundingBox()
  assert(
    Math.abs(wholeCardBefore.width - wholeCardBlockedAgain.width) < 1 &&
      Math.abs(wholeCardBefore.height - wholeCardBlockedAgain.height) < 1,
    'catalog card resized after opt-out',
  )
  await page.goto(`${base}/channel/general`)
  await page.locator('.channel-name-button').click()
  let switchControl = page.getByRole('switch', { name: 'Mark channel as NSFW' })
  assert.equal(await switchControl.isEnabled(), true)
  assert.equal(await switchControl.getAttribute('aria-checked'), 'true')
  await switchControl.click()
  await page.waitForFunction(
    () =>
      document
        .querySelector('[role="switch"][aria-label="Mark channel as NSFW"]')
        ?.getAttribute('aria-checked') === 'false',
  )
  await page.keyboard.press('Escape')
  await page.goto(`${base}/links`)
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 2)
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/first"]').count(),
    1,
  )
  await page.getByText('Connected', { exact: true }).first().waitFor()
  await page.waitForTimeout(500)
  const reclassified = await first.context.request.put(
    `${base}/api/v1/conversations/general/channel-settings`,
    { data: { nsfw: true } },
  )
  assert.equal(reclassified.status(), 200)
  await page.waitForFunction(() => document.querySelectorAll('.catalog-card-nsfw').length === 3)
  assert.equal(await page.locator('.catalog-card a').count(), 0)
  await page.goto(`${base}/channel/ergo-lounge`)
  await page.locator('.channel-name-button').click()
  switchControl = page.getByRole('switch', { name: 'Mark channel as NSFW' })
  assert.equal(await switchControl.isDisabled(), true)
  assert.equal(await page.getByText(/Changing them needs operator status/).isVisible(), true)
  cards.push(card('unknown', [{ ...source('retained-fixture', 'oftc'), nsfwKnown: false }]))
  await page.goto(`${base}/links`)
  await page.getByText('Channel settings unavailable', { exact: true }).first().waitFor()
  assert.equal(
    await page.locator('.catalog-card-title[href="https://example.invalid/unknown"]').count(),
    0,
  )
  assert.equal(
    requests.some((url) => url.includes('example.invalid/unknown')),
    false,
  )
  assert.equal(
    requests.some((url) => url.includes('/retained-fixture/')),
    false,
  )
  assert.deepEqual(errors, [])
  console.log(
    'PASS: default gate, zero preview requests, server opt-in, mixed sources, opt-out, account isolation, operator/member permissions, live reclassification, unknown classification, source navigation, demo annotations, screenshots, sanitizer',
  )
  await second.context.close()
  await first.context.close()
  assert.deepEqual(errors, [])
  if (!process.env.NSFW_CATALOG_ONLY)
    console.log(
      'PASS: retained-channel stale response fencing and two-tab private preferences events, including failed local save',
    )
} finally {
  await browser.close()
}
