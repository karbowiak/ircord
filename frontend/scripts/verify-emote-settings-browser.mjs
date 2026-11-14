// Isolated demo and HTTP fixtures: no calls mutate a real IRCord catalog.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { createServer } from 'node:net'
import { spawn } from 'node:child_process'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { readFile, readdir } from 'node:fs/promises'
import { chromium } from 'playwright'

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
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
    overrides: { buildDir: '.data/nuxt-emote-fixture', typesDir: '.data/nuxt-emote-fixture', nitro: { devStorage: { data: { driver: 'memory' } } } },
  })
`,
  ],
  {
    cwd: frontendDir,
    env: { ...process.env, NUXT_DEMO: 'true', NUXT_BACKEND_URL: 'http://127.0.0.1:1' },
    stdio: ['ignore', 'pipe', 'pipe'],
  },
)
let log = ''
for (const stream of [frontend.stdout, frontend.stderr])
  stream.on('data', (chunk) => {
    log = (log + chunk).slice(-4000)
  })
let browser
const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==',
  'base64',
)
const avatarDir = resolve(frontendDir, 'public/avatars/defaults')
const avatarFiles = (await readdir(avatarDir))
  .filter((name) => name.endsWith('.png'))
  .sort()
  .slice(0, 12)
const avatarImages = await Promise.all(
  avatarFiles.map((name) => readFile(resolve(avatarDir, name))),
)
const filters = {
  sources: ['emoji.gg'],
  categories: [],
  animations: ['all', 'static', 'animated'],
}
const item = (id, name, installed = false) => ({
  provider: 'emoji.gg',
  providerId: id,
  name,
  creator: 'Creator',
  sourceUrl: `https://emoji.gg/emoji/${id}`,
  previewUrl: `/api/v1/emotes/discover/emoji.gg/${id}/preview`,
  animated: true,
  width: 0,
  height: 0,
  category: '1',
  installed,
  license: '1',
})
const emote = (id, canManage) => ({
  id,
  name: id,
  shortcode: id,
  aliases: [],
  source: 'upload',
  creator: '',
  canManage,
  enabled: true,
  animated: false,
  url: '/fixture.png',
  staticUrl: '/fixture.png',
  width: 1,
  height: 1,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  retired: [],
})
const catalog = {
  canAdd: true,
  canManage: false,
  maxBytes: 1024 * 1024,
  types: ['image/png', 'image/gif'],
  providers: ['7tv', 'ffz', 'emoji.gg'],
  version: 1,
  items: [emote('owned', true), emote('other', false)],
}
let uploads = 0
let rejectCursor = false
let importedBody
const discoveryRequests = []
const previewRequests = []
const previewPaths = []
const errors = []
try {
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(base)).ok) break
    } catch {
      /* Starting. */
    }
    assert(attempt < 180 && frontend.exitCode === null, `Demo did not start: ${log}`)
    await new Promise((done) => setTimeout(done, 100))
  }
  browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `emote${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const page = await context.newPage()
  page.setDefaultTimeout(10000)
  page.on('pageerror', (error) => errors.push(error.message))
  await page.addInitScript(() => {
    window.__emoteUrls = { created: [], revoked: [] }
    const create = URL.createObjectURL.bind(URL)
    const revoke = URL.revokeObjectURL.bind(URL)
    URL.createObjectURL = (blob) => {
      const url = create(blob)
      window.__emoteUrls.created.push(url)
      return url
    }
    URL.revokeObjectURL = (url) => {
      window.__emoteUrls.revoked.push(url)
      revoke(url)
    }
  })
  await page.route('**/*', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (url.pathname === '/fixture.png' || url.pathname.endsWith('/preview')) {
      if (url.pathname.endsWith('/preview')) {
        previewRequests.push(url.search)
        previewPaths.push(url.pathname)
      }
      if (url.pathname.endsWith('/sample-11/preview'))
        return route.fulfill({ status: 503, contentType: 'text/plain', body: 'Unavailable' })
      const sample = url.pathname.match(/sample-(\d+)\/preview$/)
      return route.fulfill({
        status: 200,
        contentType: 'image/png',
        body: sample ? avatarImages[Number(sample[1]) % avatarImages.length] : png,
      })
    }
    if (url.pathname === '/api/v1/emotes' && request.method() === 'GET')
      return route.fulfill({ json: catalog })
    if (url.pathname === '/api/v1/emotes' && request.method() === 'POST') {
      uploads++
      if (uploads === 1)
        return route.fulfill({ status: 409, json: { detail: 'Shortcode already exists.' } })
      catalog.items.push(emote('new_upload', true))
      catalog.version++
      return route.fulfill({ json: catalog.items.at(-1) })
    }
    if (url.pathname === '/api/v1/emotes/discover') {
      discoveryRequests.push(url.searchParams.toString())
      if (url.searchParams.has('cursor') && rejectCursor) {
        rejectCursor = false
        return route.fulfill({ status: 422, json: { detail: 'Discovery cursor expired.' } })
      }
      const query = url.searchParams.get('query') || ''
      if (url.searchParams.has('category'))
        return route.fulfill({
          status: 422,
          json: { detail: 'Category is not supported in search.' },
        })
      if (query === 'slow') await new Promise((done) => setTimeout(done, 800))
      const rows =
        query === 'fast' || query === 'happy cat'
          ? [item('fast', 'Fast')]
          : query === 'filtered'
            ? url.searchParams.get('cursor')
              ? [item('slug-face', 'Slug Face')]
              : []
            : url.searchParams.get('cursor')
              ? [item('3', 'Third')]
              : [
                  item('happy-face', 'First'),
                  item('2', 'Second', true),
                  ...Array.from({ length: 12 }, (_, index) =>
                    item(
                      `sample-${index}`,
                      index === 11 ? 'AnExtraordinarilyLongEmojiName' : `Sample${index}`,
                    ),
                  ),
                ]
      return route
        .fulfill({
          json: {
            filters: query ? { ...filters, categories: [] } : filters,
            items: rows,
            nextCursor:
              query === 'filtered' && !url.searchParams.get('cursor')
                ? 'next'
                : query || url.searchParams.get('cursor')
                  ? ''
                  : 'next',
            stale: false,
          },
        })
        .catch(() => {})
    }
    if (url.pathname === '/api/v1/emotes/imports' && request.method() === 'POST') {
      const body = request.postDataJSON()
      importedBody = body
      catalog.items.push({
        ...emote('imported', true),
        source: 'emoji.gg',
        providerId: body.providerId,
      })
      catalog.version++
      return route.fulfill({ json: catalog.items.at(-1) })
    }
    return route.continue()
  })
  await page.goto(base)
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$pinia?._s.get('chat')
        ?.status === 'connected',
  )
  await page.getByRole('button', { name: 'User settings' }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings.getByRole('combobox', { name: 'Search settings' }).fill('upload emote')
  await settings.getByRole('option', { name: /Add an emote/ }).click()
  await settings.locator('input#emote-file').waitFor()
  assert.equal(
    await settings
      .locator('input#emote-file')
      .evaluate((input) => document.activeElement === input),
    true,
  )
  await settings.getByRole('button', { name: 'Close', exact: true }).last().click()
  await settings.getByText(':owned:', { exact: true }).waitFor()
  assert.equal(await settings.getByLabel('Manage :owned:').count(), 1)
  assert.equal(await settings.getByLabel('Manage :other:').count(), 0)
  await settings.getByLabel('Manage :owned:').click()
  assert.equal(await settings.getByRole('button', { name: 'Rename' }).count(), 1)
  assert.equal(
    await settings
      .locator('.emote-row')
      .filter({ hasText: ':other:' })
      .getByRole('button', { name: 'Rename' })
      .count(),
    0,
  )
  const refreshCatalog = () =>
    page.evaluate(() =>
      document
        .querySelector('#__nuxt')
        .__vue_app__.config.globalProperties.$pinia._s.get('_pc_query')
        .invalidateQueries({ key: ['emoji', 'instance'] }),
    )
  delete catalog.canAdd
  for (const row of catalog.items) delete row.canManage
  await refreshCatalog()
  assert.equal(await settings.getByRole('button', { name: 'Upload image', exact: true }).count(), 0)
  assert.equal(await settings.getByLabel('Manage :owned:').count(), 0)
  catalog.canManage = true
  await refreshCatalog()
  assert.equal(await settings.getByLabel('Manage :other:').count(), 1)
  catalog.canAdd = true
  catalog.canManage = false
  catalog.items[0].canManage = true
  catalog.items[1].canManage = false
  await refreshCatalog()
  await settings.getByLabel('Search server emoji').fill('owned')
  assert.equal(await settings.locator('.emote-row').count(), 1)
  await settings.getByLabel('Search server emoji').fill('')
  await settings.getByRole('button', { name: 'Upload image', exact: true }).first().click()
  const input = settings.locator('input[type=file]')
  await input.setInputFiles({
    name: 'invalid.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('not an image'),
  })
  await settings.getByText('This image type is not supported. Choose a supported file.').waitFor()
  assert.equal(await settings.locator('.emote-preview-image').count(), 0)
  await input.setInputFiles({
    name: 'large.png',
    mimeType: 'image/png',
    buffer: Buffer.alloc(1024 * 1024 + 1),
  })
  await settings.getByText('The image is too large. Choose a file up to 1,024 KiB.').waitFor()
  assert.equal(await settings.locator('.emote-preview-image').count(), 0)
  await input.setInputFiles({ name: 'party.png', mimeType: 'image/png', buffer: png })
  await settings.locator('.emote-chat-preview img').waitFor()
  assert.equal(await settings.locator('#emote-shortcode').inputValue(), 'party')
  const firstPreview = await settings.locator('.emote-preview-image').getAttribute('src')
  await input.setInputFiles({ name: 'another.png', mimeType: 'image/png', buffer: png })
  assert(await page.evaluate((url) => window.__emoteUrls.revoked.includes(url), firstPreview))
  assert.equal(await settings.locator('#emote-shortcode').inputValue(), 'party')
  const dimensions = await settings.locator('.emote-chat-preview img').evaluate((el) => ({
    width: el.getBoundingClientRect().width,
    font: parseFloat(getComputedStyle(el).fontSize),
  }))
  assert(Math.abs(dimensions.width / dimensions.font - 1.6) < 0.05)
  const serverGeometry = await settings.locator('.emote-settings').evaluate((root) => {
    const search = root.querySelector('.emote-search').getBoundingClientRect()
    const list = root.querySelector('.emote-content').getBoundingClientRect()
    const panel = root.querySelector('.emote-detail').getBoundingClientRect()
    return { search: search.width, list: list.width, listRight: list.right, panelLeft: panel.left }
  })
  assert(
    serverGeometry.search > serverGeometry.list &&
      serverGeometry.panelLeft > serverGeometry.listRight,
  )
  await page.screenshot({ path: '../.data/emote-refinement-server.png' })
  await settings.locator('#emote-shortcode').fill('party_custom')
  await settings.locator('#emote-aliases').fill('party_alias')
  await settings.getByRole('radio', { name: 'Discover' }).click()
  assert.equal(await settings.locator('.emote-detail').count(), 0)
  await settings.getByRole('radio', { name: 'Server emoji' }).click()
  await settings.getByRole('button', { name: 'Upload image', exact: true }).first().click()
  assert.equal(await settings.locator('#emote-shortcode').inputValue(), 'party_custom')
  assert.equal(await settings.locator('#emote-aliases').inputValue(), 'party_alias')
  await settings.getByRole('button', { name: 'Add to server' }).click()
  await settings.getByText('Shortcode already exists.').waitFor()
  assert.equal(await settings.locator('#emote-shortcode').inputValue(), 'party_custom')
  assert.equal(await settings.locator('#emote-aliases').inputValue(), 'party_alias')
  await settings.getByRole('button', { name: 'Add to server' }).click()
  await settings.getByText('Emote added.').waitFor()
  await settings.getByText(':new_upload:', { exact: true }).waitFor()
  assert.equal(uploads, 2)
  const urls = await page.evaluate(() => window.__emoteUrls)
  assert(urls.created.length > 0 && urls.revoked.length > 0)
  await settings.getByRole('button', { name: 'Close', exact: true }).last().click()
  await settings.getByRole('radio', { name: 'Server emoji' }).focus()
  await page.keyboard.press('ArrowRight')
  await settings.getByRole('radio', { name: 'Discover', checked: true }).waitFor()
  await settings.getByRole('button', { name: /First/ }).waitFor()
  assert.equal(await settings.locator('.emote-provider-pill').textContent(), 'emoji.gg')
  assert.equal(await settings.getByLabel('Category', { exact: true }).count(), 0)
  const animationResponse = page.waitForResponse(
    (response) =>
      response.url().includes('/emotes/discover?') &&
      new URL(response.url()).searchParams.get('animation') === 'animated',
  )
  await settings.getByRole('radio', { name: 'Animated', exact: true }).click()
  await animationResponse
  const allAnimationResponse = page.waitForResponse(
    (response) =>
      response.url().includes('/emotes/discover?') &&
      new URL(response.url()).searchParams.get('animation') === 'all',
  )
  await settings.getByRole('radio', { name: 'All', exact: true }).click()
  await allAnimationResponse
  await page.waitForFunction(() => {
    const tiles = [...document.querySelectorAll('.emote-tile')]
    const images = tiles.slice(0, 2).map((tile) => tile.querySelector('img'))
    return tiles.length === 14 && images.every((image) => image?.complete && image.naturalWidth > 0)
  })
  assert(previewRequests.every((search) => search.includes('static=true')))
  assert.equal(
    await settings
      .getByRole('button', { name: /Second/ })
      .getByText('Installed')
      .count(),
    1,
  )
  const failedName = 'AnExtraordinarilyLongEmojiName'
  const failedTile = settings.locator('.emote-tile').filter({ hasText: failedName })
  await failedTile.scrollIntoViewIfNeeded()
  await failedTile.locator('.emote-fallback').waitFor()
  async function assertClippedFallback(locator, label) {
    const result = await locator.evaluate((element) => {
      const parent = element.parentElement
      const outer = parent.getBoundingClientRect()
      const child = element.getBoundingClientRect()
      const style = getComputedStyle(element)
      return {
        label: element.getAttribute('aria-label'),
        parentOverflow: getComputedStyle(parent).overflow,
        childOverflow: style.overflow,
        textOverflow: style.textOverflow,
        whiteSpace: style.whiteSpace,
        width: outer.width,
        height: outer.height,
        fontSize: parseFloat(getComputedStyle(parent).fontSize),
        scrollWidth: element.scrollWidth,
        clientWidth: element.clientWidth,
        within:
          child.left >= outer.left - 0.5 &&
          child.right <= outer.right + 0.5 &&
          child.top >= outer.top - 0.5 &&
          child.bottom <= outer.bottom + 0.5,
      }
    })
    assert.equal(result.label, label)
    assert.equal(result.within, true)
    assert.equal(result.parentOverflow, 'hidden')
    assert.equal(result.childOverflow, 'hidden')
    assert.equal(result.textOverflow, 'ellipsis')
    assert.equal(result.whiteSpace, 'nowrap')
    assert(result.scrollWidth > result.clientWidth, 'Long fallback must be visibly clipped')
    return result
  }
  const gridFallback = await assertClippedFallback(
    failedTile.locator('.emote-fallback'),
    failedName,
  )
  const normalGrid = await settings
    .getByRole('button', { name: /First/ })
    .locator('.emote-still')
    .boundingBox()
  assert(
    Math.abs(gridFallback.width - normalGrid.width) < 0.5 &&
      Math.abs(gridFallback.height - normalGrid.height) < 0.5,
  )
  await failedTile.click()
  const detailFallback = settings.locator('.emote-preview-art .emote-fallback')
  const inlineFallback = settings.locator('.emote-chat-preview .emote-fallback')
  await detailFallback.waitFor()
  await inlineFallback.waitFor()
  const detailGeometry = await assertClippedFallback(detailFallback, failedName)
  const inlineGeometry = await assertClippedFallback(inlineFallback, failedName.toLowerCase())
  assert(detailGeometry.width > gridFallback.width && detailGeometry.height > gridFallback.height)
  assert(Math.abs(inlineGeometry.width / inlineGeometry.fontSize - 1.6) < 0.05)
  assert(Math.abs(inlineGeometry.height / inlineGeometry.fontSize - 1.6) < 0.05)
  await page.screenshot({ path: '../.data/emote-failed-preview.png' })
  await settings.getByRole('button', { name: /First/ }).click()
  assert.equal(
    await settings
      .locator('.emote-detail-heading h3')
      .evaluate((element) => document.activeElement === element),
    true,
  )
  assert(previewPaths.some((path) => path.includes('/happy-face/preview')))
  assert.equal(
    await settings.getByRole('link', { name: 'Usage terms' }).getAttribute('href'),
    'https://emoji.gg/licenses',
  )
  assert.equal(await settings.getByText('License: 1').count(), 0)
  assert.equal(await settings.getByText(/by Creator/).count(), 1)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.waitForFunction(() =>
    document
      .querySelector('.emote-provider-preview img')
      ?.getAttribute('src')
      ?.includes('static=true'),
  )
  await settings.locator('#emote-shortcode').fill('first_custom')
  await settings.locator('#emote-aliases').fill('first_alias')
  const discoveryGeometry = await settings.locator('.emote-settings').evaluate((root) => {
    const search = root.querySelector('.emote-search').getBoundingClientRect()
    const grid = root.querySelector('.emote-grid').getBoundingClientRect()
    const panel = root.querySelector('.emote-detail').getBoundingClientRect()
    return {
      search: search.width,
      grid: grid.width,
      gridRight: grid.right,
      panelLeft: panel.left,
      columns: getComputedStyle(root.querySelector('.emote-grid')).gridTemplateColumns.split(' ')
        .length,
    }
  })
  assert(
    discoveryGeometry.search > discoveryGeometry.grid &&
      discoveryGeometry.panelLeft > discoveryGeometry.gridRight &&
      discoveryGeometry.columns >= 3,
  )
  assert.equal(
    await settings
      .locator('.emote-grid')
      .evaluate((element) => element.scrollWidth <= element.clientWidth),
    true,
  )
  await page.screenshot({ path: '../.data/emote-refinement-discover.png' })
  await settings.getByRole('button', { name: 'Add to server' }).click()
  assert.equal(importedBody.providerId, 'happy-face')
  await settings.getByText('Emote imported.').waitFor()
  await settings.getByRole('button', { name: 'Installed', exact: true }).waitFor()
  await page.screenshot({ path: '../.data/emote-settings-desktop.png' })
  await settings.getByRole('button', { name: 'Close', exact: true }).last().click()
  await settings.getByRole('button', { name: 'Next', exact: true }).click()
  await settings.getByRole('button', { name: /Third/ }).waitFor()
  assert(
    discoveryRequests.some(
      (request) => request.includes('cursor=next') && request.includes('limit=30'),
    ),
  )
  await settings.getByRole('button', { name: 'Previous', exact: true }).click()
  await settings.getByRole('button', { name: /First/ }).waitFor()
  assert.equal(await settings.getByLabel('Category', { exact: true }).count(), 0)
  rejectCursor = true
  await settings.getByRole('button', { name: 'Next', exact: true }).click()
  await settings.getByText('Discovery cursor expired.').waitFor()
  assert.equal(await settings.getByText('Emoji could not load.').count(), 1)
  await page.screenshot({ path: '../.data/emote-refinement-error.png' })
  await settings.getByRole('button', { name: 'Try again' }).click()
  await settings.getByRole('button', { name: /First/ }).waitFor()
  const search = settings.getByLabel('Search emoji')
  await search.fill('slow')
  await page.waitForTimeout(350)
  assert.equal(await settings.getByRole('status', { name: 'Loading emoji…' }).isVisible(), true)
  assert.equal(await settings.getByLabel('Category', { exact: true }).count(), 0)
  await search.fill('happy cat')
  await settings.getByRole('button', { name: /Fast/ }).waitFor()
  await page.waitForTimeout(900)
  assert.equal(await settings.getByRole('button', { name: /Fast/ }).count(), 1)
  assert(
    discoveryRequests
      .filter((request) => request.includes('query=slow') || request.includes('query=happy'))
      .every((request) => !request.includes('category=')),
  )
  await search.fill('filtered')
  await settings.getByText('No matches on this page.').waitFor()
  await page.screenshot({ path: '../.data/emote-refinement-empty.png' })
  assert.equal(await settings.getByRole('button', { name: 'Next', exact: true }).isEnabled(), true)
  await settings.getByRole('button', { name: 'Next', exact: true }).click()
  await settings.getByRole('button', { name: /Slug Face/ }).waitFor()
  const browseResponse = page.waitForResponse(
    (response) =>
      response.url().includes('/emotes/discover?') &&
      !new URL(response.url()).searchParams.has('query'),
  )
  await search.fill('')
  await browseResponse
  assert.equal(await settings.getByLabel('Category', { exact: true }).count(), 0)
  assert(discoveryRequests.every((request) => !request.includes('category=')))
  await search.fill('fast')
  await settings.getByRole('button', { name: /Fast/ }).waitFor()
  await page.setViewportSize({ width: 390, height: 844 })
  await settings.getByRole('button', { name: /Fast/ }).click()
  await page.waitForFunction(
    () => document.querySelector('.emote-provider-preview img')?.naturalWidth > 0,
  )
  await page.screenshot({ path: '../.data/emote-refinement-phone.png' })
  assert.equal(await settings.locator('.emote-content').isVisible(), false)
  assert.equal(await settings.locator('.emote-detail').isVisible(), true)
  assert.equal(
    await settings
      .locator('.emote-settings')
      .evaluate((element) => element.scrollWidth <= element.clientWidth),
    true,
  )
  await settings.getByRole('button', { name: 'Close', exact: true }).last().click()
  assert.equal(
    await settings
      .getByRole('button', { name: /Fast/ })
      .evaluate((element) => document.activeElement === element),
    true,
  )
  await settings.getByRole('button', { name: /Fast/ }).click()
  await page.waitForFunction(
    () => history.state?.ircordViews?.values?.['emote-panel'] === 'discover',
  )
  await page.evaluate(() => history.back())
  await settings.locator('.emote-detail').waitFor({ state: 'hidden' })
  assert.equal(await settings.locator('.emote-content').isVisible(), true)
  await page.evaluate(() => history.forward())
  await settings.locator('.emote-detail').waitFor({ state: 'visible' })
  await settings.getByRole('button', { name: 'Profile', exact: true }).click()
  await page.evaluate(() => history.back())
  await settings.locator('.emote-settings').waitFor()
  if (await settings.locator('.emote-detail').count())
    assert.equal(await settings.locator('#emote-shortcode').inputValue(), 'fast')
  assert.deepEqual(errors, [])
  console.log('Emote settings disposable browser fixture passed')
} finally {
  await browser?.close()
  frontend.kill('SIGTERM')
}
