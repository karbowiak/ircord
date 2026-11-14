// Isolated demo UI check for retained and bounded upstream search surfaces.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { createServer } from 'node:net'
import { spawn } from 'node:child_process'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const frontendDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const probe = createServer()
await new Promise((resolve) => probe.listen(0, '127.0.0.1', resolve))
const port = probe.address().port
await new Promise((resolve) => probe.close(resolve))
const base = `http://127.0.0.1:${port}`
const frontend = spawn(
  process.execPath,
  [
    '-e',
    `
      import { runCommand } from '@nuxt/cli'
      await runCommand('dev', ['--host', '127.0.0.1', '--port', '${port}', '--strictPort', '--no-fork', '--no-tui', '--no-takeover'], {
        overrides: {
          buildDir: '.data/nuxt-search-source',
          typesDir: '.data/nuxt-search-source',
          nitro: { devStorage: { data: { driver: 'memory' } } },
        },
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
  stream.on('data', (chunk) => (serverLog = (serverLog + chunk).slice(-4000)))
let browser
let context
try {
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(base)).ok) break
    } catch {
      // The isolated demo is still starting.
    }
    assert(attempt < 180 && frontend.exitCode === null, `Demo did not start: ${serverLog}`)
    await new Promise((resolve) => setTimeout(resolve, 100))
  }

  browser = await chromium.launch({ headless: true })
  context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `search${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  assert.equal(bootstrap.demo, true, 'The fixture must never use the Go backend')
  const conversation = bootstrap.conversations.find((item) => item.name === 'general')
  assert(conversation, 'The disposable demo must include #general')

  const requests = []
  const page = await context.newPage()
  page.setDefaultTimeout(7000)
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  const mockSearch = async (route) => {
    const url = new URL(route.request().url())
    const requested = url.searchParams.get('source')
    requests.push({ requested, conversationId: url.searchParams.getAll('conversationId') })
    const upstream = requested === 'upstream'
    const message = {
      id: 'search:ephemeral-check',
      conversationId: conversation.id,
      senderId: 'search:sender:synthetic-id-must-not-render',
      text: 'server-only verification result',
      createdAt: new Date().toISOString(),
      kind: 'message',
      reactions: [],
      historical: true,
      upstreamId: 'upstream-only',
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        items: upstream
          ? [
              {
                message,
                conversation,
                senderNick: 'RemoteNick',
                context: { available: false, identity: 'ephemeral', reason: 'not-retained' },
              },
            ]
          : [],
        nextCursor: null,
        source: {
          requested,
          actual: upstream ? 'upstream' : 'retained',
          coverage: upstream ? 'upstream-bounded' : 'retained-indexed',
          completeness: 'unknown',
        },
      }),
    })
  }
  await page.route('**/api/v1/search**', mockSearch)

  await page.goto(`${base}/search`)
  const workspaceSearch = page.getByRole('combobox', { name: 'Search workspace' })
  await workspaceSearch.fill('server')
  await page.waitForResponse(
    (response) =>
      response.url().includes('/api/v1/search') &&
      new URL(response.url()).searchParams.get('q') === 'server',
  )
  assert.equal(requests.at(-1)?.requested, 'retained')
  await page.getByRole('tab', { name: /^Messages/ }).click()
  await page
    .getByText('Coverage: messages retained by IRCord; server history may include more.', {
      exact: true,
    })
    .waitFor()
  assert.equal(await page.getByRole('switch', { name: 'Search server history' }).isDisabled(), true)
  await page.screenshot({ path: '../.data/phase4-search-page.png', fullPage: true })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: '../.data/phase4-search-page-mobile.png', fullPage: true })
  await page.setViewportSize({ width: 1440, height: 900 })

  const conversationPage = await context.newPage()
  conversationPage.setDefaultTimeout(7000)
  conversationPage.on('pageerror', (error) => errors.push(error.message))
  await conversationPage.route('**/api/v1/search**', mockSearch)
  await conversationPage.goto(`${base}/channel/general`)
  await conversationPage
    .getByRole('button', { name: 'Search this conversation', exact: true })
    .click()
  const conversationSearch = conversationPage.getByRole('combobox', {
    name: 'Search messages in #general',
  })
  await conversationSearch.fill('remote')
  await conversationPage.waitForResponse(
    (response) =>
      response.url().includes('/api/v1/search') &&
      new URL(response.url()).searchParams.get('q') === 'remote',
  )
  assert.equal(requests.at(-1)?.requested, 'retained')
  const sourceSwitch = conversationPage.getByRole('switch', { name: 'Search server history' })
  assert.equal(await sourceSwitch.isDisabled(), false)
  await sourceSwitch.click()
  await conversationPage.waitForResponse(
    (response) =>
      response.url().includes('/api/v1/search') &&
      new URL(response.url()).searchParams.get('source') === 'upstream',
  )
  assert.equal(await sourceSwitch.getAttribute('aria-checked'), 'true')
  assert.equal(requests.at(-1)?.requested, 'upstream')
  await conversationPage
    .getByText('Server results are bounded and may not include every match.', { exact: true })
    .waitFor()
  const remoteResult = conversationPage.locator('.conversation-search .search-message')
  await remoteResult.waitFor()
  assert.equal(await remoteResult.getAttribute('aria-disabled'), 'true')
  assert.match(await remoteResult.innerText(), /RemoteNick/)
  assert.doesNotMatch(await remoteResult.innerText(), /synthetic-id-must-not-render/)
  assert.equal(
    await conversationPage
      .getByRole('button', { name: 'Load older messages', exact: true })
      .count(),
    0,
  )
  assert.equal(await remoteResult.evaluate((button) => button.disabled), false)
  await remoteResult.evaluate((button) => button.focus())
  await conversationPage.keyboard.press('Enter')
  await remoteResult.evaluate((button) => button.click())
  assert.equal(
    await conversationPage.locator('.conversation-search').count(),
    1,
    'An ephemeral hit must not reveal a timeline message',
  )
  await conversationPage.screenshot({
    path: '../.data/phase4-conversation-search.png',
    fullPage: true,
  })
  const mobilePage = await context.newPage()
  mobilePage.setDefaultTimeout(7000)
  mobilePage.on('pageerror', (error) => errors.push(error.message))
  mobilePage.setViewportSize({ width: 390, height: 844 })
  await mobilePage.route('**/api/v1/search**', mockSearch)
  await mobilePage.goto(`${base}/channel/general`)
  await mobilePage.getByRole('button', { name: 'Search this conversation', exact: true }).click()
  const mobileSearch = mobilePage.getByRole('combobox', { name: 'Search messages in #general' })
  await mobileSearch.fill('mobile')
  await mobilePage.waitForResponse(
    (response) =>
      response.url().includes('/api/v1/search') &&
      new URL(response.url()).searchParams.get('q') === 'mobile',
  )
  await mobilePage.getByRole('switch', { name: 'Search server history' }).click()
  await mobilePage.waitForResponse(
    (response) =>
      response.url().includes('/api/v1/search') &&
      new URL(response.url()).searchParams.get('source') === 'upstream' &&
      new URL(response.url()).searchParams.get('q') === 'mobile',
  )
  assert.equal(
    await mobilePage
      .getByRole('switch', { name: 'Search server history' })
      .getAttribute('aria-checked'),
    'true',
  )
  await page.waitForTimeout(200)
  await mobilePage.screenshot({
    path: '../.data/phase4-conversation-search-mobile.png',
    fullPage: true,
  })
  assert.deepEqual(errors, [])
  console.log(
    'Search browser fixture passed: global retained source, scoped upstream source, bounded note, and non-jumpable ephemeral hit.',
  )
} finally {
  await context?.close()
  await browser?.close()
  if (frontend.exitCode === null) {
    const exited = new Promise((resolve) => frontend.once('exit', resolve))
    frontend.kill('SIGTERM')
    await exited
  }
}
