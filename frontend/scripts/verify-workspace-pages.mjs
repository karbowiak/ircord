// Uses its own production demo and disposable account; never targets the live backend.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { createServer } from 'node:net'
import { mkdir } from 'node:fs/promises'
import { chromium } from 'playwright'
import { createIRCordClient } from '../shared/api/client.ts'
import { normalizeTransport, transportMemberId } from '../app/utils/transport.ts'

// Live transport uses account IDs; UI filters keep the stable self sentinel.
normalizeTransport({ user: { id: 'search-account' } })
let sentSearch
const client = createIRCordClient('http://127.0.0.1:1/api/v1', {
  memberId: transportMemberId,
  fetch: async (request) => {
    sentSearch = new URL(request.url).searchParams
    return Response.json({ items: [], nextCursor: null })
  },
})
await client.search({
  from: ['you', 'peer'],
  mentions: ['you', 'other'],
  conversationId: ['one', 'two'],
  has: ['link', 'image'],
})
assert.deepEqual(sentSearch.getAll('from'), ['search-account', 'peer'])
assert.deepEqual(sentSearch.getAll('mentions'), ['search-account', 'other'])
assert.deepEqual(sentSearch.getAll('conversationId'), ['one', 'two'])
assert.deepEqual(sentSearch.getAll('has'), ['link', 'image'])
normalizeTransport({ user: { id: 'you' } })
assert.equal(transportMemberId('you'), 'you')

const portProbe = createServer()
await new Promise((resolve) => portProbe.listen(0, '127.0.0.1', resolve))
const port = portProbe.address().port
await new Promise((resolve) => portProbe.close(resolve))
const base = `http://127.0.0.1:${port}`
const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
  env: { ...process.env, HOST: '127.0.0.1', PORT: String(port), NUXT_DEMO: 'true' },
  stdio: ['ignore', 'pipe', 'pipe'],
})
let serverLog = ''
frontend.stdout.on('data', (chunk) => {
  serverLog = (serverLog + chunk).slice(-4000)
})
frontend.stderr.on('data', (chunk) => {
  serverLog = (serverLog + chunk).slice(-4000)
})
let browser
let context
let created = false
const errors = []
try {
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(base)).ok) break
    } catch {
      // The server is not accepting connections yet; retry below.
    }
    assert(attempt < 100 && frontend.exitCode === null, 'Isolated frontend did not start')
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  browser = await chromium.launch({ headless: true })
  context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `pages${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const { account } = await signup.json()
  created = true
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  assert.equal(bootstrap.demo, true)
  const page = await context.newPage()
  page.setDefaultTimeout(5000)
  page.on('pageerror', (error) => errors.push(error.message))
  const sidebar = page.locator('.conversation-sidebar')
  async function tool(label, path) {
    await sidebar
      .getByRole('button', { name: new RegExp(`^${label}`) })
      .first()
      .click()
    await page.waitForURL(`${base}${path}`)
    await page.locator('.workspace-page h1').waitFor()
  }
  await page.goto(base)
  await page.waitForURL(`${base}/channel/general`)
  const composer = page.getByRole('textbox', { name: 'Message #general', exact: true })
  const draft = 'Keep this draft while browsing workspace pages'
  await composer.fill(draft)
  const scripts = () =>
    page.evaluate(() =>
      performance
        .getEntriesByType('resource')
        .filter((entry) => /\/_nuxt\/.*\.js(?:\?|$)/.test(entry.name))
        .map((entry) => entry.name),
    )
  const initialScripts = await scripts()
  for (const [label, path] of [
    ['Search', '/search'],
    ['Inbox', '/inbox'],
    ['Saved', '/saved'],
    ['Drafts', '/drafts'],
    ['Links', '/links'],
  ]) {
    await tool(label, path)
    assert.equal(await page.locator('.composer').count(), 0, `${label} replaces the conversation`)
    assert.equal(await page.getByRole('dialog').count(), 0, `${label} is a page`)
    assert.equal(await sidebar.locator('[aria-current="page"]').count(), 1, `${label} is selected`)
  }
  assert.deepEqual(
    await scripts(),
    initialScripts,
    'Pages load with the app, without extra JavaScript downloads',
  )
  await page.getByRole('button', { name: 'Return to conversation', exact: true }).click()
  await page.waitForURL(`${base}/channel/general`)
  assert.equal(await composer.inputValue(), draft)
  await tool('Drafts', '/drafts')
  await page.locator('.draft-open').filter({ hasText: '#general' }).click()
  await page.waitForURL(`${base}/channel/general`)
  await page.waitForFunction(() => document.activeElement?.tagName === 'TEXTAREA')
  assert.equal(await composer.inputValue(), draft)

  await tool('Search', '/search')
  await page.getByRole('combobox', { name: 'Search workspace' }).fill('general')
  await tool('Saved', '/saved')
  await page.goBack()
  await page.waitForURL(`${base}/search`)
  assert.equal(
    await page.getByRole('combobox', { name: 'Search workspace' }).inputValue(),
    'general',
  )
  await page.goForward()
  await page.waitForURL(`${base}/saved`)
  await page.reload()
  await page.getByRole('heading', { name: 'Saved messages', exact: true }).waitFor()

  // Startup must not replace a utility page when the last conversation has disappeared.
  await page.evaluate((accountId) => {
    sessionStorage.setItem(`ircord.${accountId}.active`, JSON.stringify('closed-conversation'))
  }, account.id)
  await page.goto(`${base}/search`)
  await page.getByRole('combobox', { name: 'Search workspace' }).waitFor()
  assert.equal(new URL(page.url()).pathname, '/search')
  await page.getByRole('combobox', { name: 'Search workspace' }).fill('general')
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  assert.equal(await page.getByRole('combobox', { name: 'Search workspace' }).inputValue(), '')

  await tool('Links', '/links')
  assert.equal(await page.getByRole('textbox', { name: 'Host', exact: true }).isVisible(), false)
  await page.getByRole('button', { name: /^Filters/ }).click()
  await page.getByRole('textbox', { name: 'Host', exact: true }).fill('no-match.invalid')
  await page.getByText('No links match these filters.', { exact: true }).waitFor()
  await page.getByRole('button', { name: 'Clear filters', exact: true }).first().click()
  await page.locator('.catalog-card').first().waitFor()

  await tool('Drafts', '/drafts')
  await page.getByRole('button', { name: /^Discard draft in #general/ }).click()
  assert.equal(await page.locator('.draft-row').count(), 0)
  await page.getByRole('button', { name: 'Undo', exact: true }).click()
  await page.locator('.draft-open').filter({ hasText: '#general' }).waitFor()

  await page.getByRole('button', { name: 'Return to conversation', exact: true }).click()
  await page.waitForURL(`${base}/channel/general`)
  await page.locator('.network-rail').getByRole('button', { name: 'Ergo', exact: true }).click()
  await page.waitForURL(`${base}/channel/ergo-lounge`)
  await sidebar.getByRole('button', { name: 'Server', exact: true }).click()
  await page.waitForURL(`${base}/server/ergo`)
  await page.locator('.server-window').waitFor()
  // Summary cards, the security details and the capabilities column.
  await page.locator('.latency-chart').waitFor()
  await page.getByRole('button', { name: /^Security A/ }).click()
  await page
    .getByRole('dialog', { name: 'Connection security' })
    .getByText('Protocol', { exact: true })
    .waitFor()
  await page.keyboard.press('Escape')
  assert.equal(new URL(page.url()).pathname, '/server/ergo', 'Escape closes only the popover')
  const extensions = page.getByRole('region', { name: 'IRCord extensions', exact: true })
  const detailsFilter = page.getByRole('textbox', { name: 'Filter server details', exact: true })
  await extensions.getByText('Available', { exact: true }).waitFor()
  const extensionToggle = extensions.getByRole('button', { name: 'IRCord extensions', exact: true })
  const sectionOrder = await page.locator('.server-capabilities h2').allTextContents()
  assert.match(sectionOrder[0], /Capabilities/)
  assert.match(sectionOrder[1], /IRCord extensions/)
  assert.match(sectionOrder[2], /ISUPPORT/)
  const capabilityToggle = page
    .locator('.server-capabilities')
    .getByRole('button', { name: 'Capabilities', exact: true })
  await capabilityToggle.click()
  assert.equal(await capabilityToggle.getAttribute('aria-expanded'), 'false')
  assert.equal(await page.locator('.server-capability').count(), 0)
  await extensions.getByText('Available', { exact: true }).waitFor()
  assert.equal(await page.locator('.server-capabilities dl').count(), 1)
  await capabilityToggle.press('Space')
  assert.equal(await capabilityToggle.getAttribute('aria-expanded'), 'true')
  assert((await page.locator('.server-capability').count()) > 0)
  await extensionToggle.click()
  assert.equal(await extensionToggle.getAttribute('aria-expanded'), 'false')
  assert.equal(await extensions.locator('.server-extension').count(), 0)
  assert.equal((await page.locator('.server-capability').count()) > 0, true)
  await extensionToggle.press('Enter')
  assert.equal(await extensionToggle.getAttribute('aria-expanded'), 'true')
  await extensions.getByText('Available', { exact: true }).waitFor()
  assert.equal(await extensions.getByText('+ircord/edit', { exact: true }).count(), 1)
  assert.equal(
    await page.locator('.server-capability').filter({ hasText: 'ircord/edit' }).count(),
    0,
  )
  for (const query of ['ircord/edit', 'message editing']) {
    await detailsFilter.fill(query)
    await extensions.getByText('Available', { exact: true }).waitFor()
    assert.equal(await page.locator('.server-capability').count(), 0)
  }
  await detailsFilter.fill('chathistory')
  assert.equal(await page.locator('.server-capability').count(), 1)
  await extensions.getByText('No IRCord extensions match.', { exact: true }).waitFor()
  await detailsFilter.fill('')
  const capabilityStatus = page.getByRole('radiogroup', { name: 'Capability status', exact: true })
  await capabilityStatus.getByRole('radio', { name: 'Announced', exact: true }).click()
  await extensions.getByText('Available', { exact: true }).waitFor()
  await capabilityStatus.getByRole('radio', { name: 'All', exact: true }).click()
  await mkdir('../.data/server-extensions-review', { recursive: true })
  await capabilityToggle.click()
  await extensionToggle.scrollIntoViewIfNeeded()
  await page.screenshot({ path: '../.data/server-extensions-review/desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await page
    .getByRole('radiogroup', { name: 'Server view', exact: true })
    .getByRole('radio', { name: 'Capabilities', exact: true })
    .click()
  await extensions.getByText('Available', { exact: true }).waitFor()
  await capabilityToggle.click()
  assert.equal(await capabilityToggle.getAttribute('aria-expanded'), 'true')
  await capabilityToggle.click()
  assert.equal(await capabilityToggle.getAttribute('aria-expanded'), 'false')
  await extensionToggle.scrollIntoViewIfNeeded()
  await extensionToggle.click()
  assert.equal(await extensionToggle.getAttribute('aria-expanded'), 'false')
  await extensionToggle.click()
  await extensions.getByText('Available', { exact: true }).waitFor()
  await page.screenshot({ path: '../.data/server-extensions-review/mobile.png' })
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
  const extensionBounds = await extensions.boundingBox()
  assert(
    extensionBounds && extensionBounds.x >= 0 && extensionBounds.x + extensionBounds.width <= 390,
  )
  await page.setViewportSize({ width: 1440, height: 900 })

  // Cached feature support must not imply availability on a disconnected network.
  const disconnected = await context.request.patch(`${base}/api/v1/networks/ergo`, {
    data: { status: 'disconnected' },
  })
  assert.equal(disconnected.status(), 200)
  assert.equal((await disconnected.json()).features.edit, true)
  await extensions.getByText('Unavailable', { exact: true }).waitFor()
  await extensions.getByText('Reconnect to use message editing.', { exact: true }).waitFor()
  const reconnected = await context.request.patch(`${base}/api/v1/networks/ergo`, {
    data: { status: 'connected' },
  })
  assert.equal(reconnected.status(), 200)
  await extensions.getByText('Available', { exact: true }).waitFor()

  await page.goto(`${base}/server/oftc`)
  await extensions.getByText('Unavailable', { exact: true }).waitFor()
  await extensions
    .getByText(
      'Requires message-tags, echo-message, account-tag, and an allowed ircord/edit tag.',
      { exact: true },
    )
    .waitFor()
  await page.goto(`${base}/server/ergo`)
  await extensions.getByText('Available', { exact: true }).waitFor()
  await page.getByRole('textbox', { name: 'Filter server lines' }).fill('does-not-match-any-line')
  assert.equal(await page.locator('.server-line').count(), 0)
  await page
    .locator('.server-toolbar')
    .getByRole('button', { name: 'Clear search', exact: true })
    .click()
  await page.locator('.server-line').first().waitFor()
  await page.reload()
  await page.locator('.server-window').waitFor()
  await page
    .locator('.network-rail')
    .getByRole('button', { name: /^Libera Chat/ })
    .click()
  await page.waitForURL(`${base}/channel/general`)
  await sidebar.getByRole('button', { name: 'Echo', exact: true }).click()
  await page.waitForURL(`${base}/dm/dm-echo`)
  assert.equal(await page.locator('.member-sidebar').count(), 0)

  await tool('Inbox', '/inbox')
  await page.getByRole('button', { name: 'Dismiss from inbox', exact: true }).first().click()
  await page.getByRole('tab', { name: 'Dismissed', exact: true }).click()
  await page.getByRole('button', { name: 'Restore to inbox', exact: true }).first().click()
  await page.getByRole('tab', { name: 'All', exact: true }).click()
  await page.locator('.inbox-body .search-message').first().click()
  await page.waitForURL(/\/(channel|dm)\//)
  await page.locator('.message-row.is-highlighted').waitFor()
  await page.goto(`${base}/#libera-chat/%23general`)
  await page.waitForURL(`${base}/channel/general`)
  assert.equal(await composer.inputValue(), draft)
  await tool('Search', '/search')
  await page.keyboard.press('Control+1')
  await page.waitForURL(`${base}/channel/development`)

  // Seed retained history in this disposable demo only; no traffic is generated.
  const seeded = await context.request.post(`${base}/api/v1/_demo/busy`, {
    data: { members: 4, history: 125, rate: 0, churn: 0, seconds: 0 },
  })
  assert.equal(seeded.status(), 200)
  await page.reload()
  await sidebar
    .getByRole('button', { name: /^Search/ })
    .first()
    .waitFor()
  await tool('Search', '/search')
  const field = page.getByRole('combobox', { name: 'Search workspace' })
  await field.fill('in:busy')
  const onlyChannel = page.waitForResponse((response) => {
    const url = new URL(response.url())
    return (
      url.pathname === '/api/v1/search' &&
      url.searchParams.get('conversationId') === 'busy' &&
      !url.searchParams.has('q')
    )
  })
  await page.getByRole('option', { name: /#busy/ }).click()
  assert.equal((await onlyChannel).status(), 200)
  const rows = page.locator('.search-results .search-message')
  await page.waitForFunction(
    () => document.querySelectorAll('.search-results .search-message').length === 50,
  )
  await page.getByRole('button', { name: 'Load older messages', exact: true }).click()
  await page.waitForFunction(
    () => document.querySelectorAll('.search-results .search-message').length === 100,
  )
  await page.getByRole('button', { name: 'Load older messages', exact: true }).click()
  await page.waitForFunction(
    () => document.querySelectorAll('.search-results .search-message').length === 125,
  )
  assert.equal(
    await page.getByRole('button', { name: 'Load older messages', exact: true }).count(),
    0,
  )
  assert.equal((await rows.allTextContents()).length, 125)
  const expectedLinks = await context.request.get(
    `${base}/api/v1/search?conversationId=busy&has=link&limit=100`,
  )
  const expected = await expectedLinks.json()
  assert(expected.items.length > 0 && expected.items.length < 50)
  await field.fill('has:link')
  await page.getByRole('option', { name: 'Link', exact: true }).click()
  await page.waitForFunction(
    (count) => document.querySelectorAll('.search-results .search-message').length === count,
    expected.items.length,
  )
  assert.equal(await field.inputValue(), '')
  assert.equal(await page.getByText(/Filters are applied to the 50 newest/).count(), 0)
  await page.getByRole('heading', { name: 'Search', exact: true }).click()
  await mkdir('../.data/search-review', { recursive: true })
  await page.screenshot({ path: '../.data/search-review/search-desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: '../.data/search-review/search-mobile.png' })
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
  await page.setViewportSize({ width: 1440, height: 900 })
  assert.deepEqual(errors, [])
  console.log(
    'Workspace pages, drafts, history, navigation, search filters, filter-only search and older result pages pass.',
  )
} catch (error) {
  // Server diagnostics exclude configuration and request bodies.
  if (serverLog) console.error(serverLog)
  throw error
} finally {
  if (created) await context?.request.delete(`${base}/api/v1/auth/account`).catch(() => {})
  await browser?.close()
  frontend.kill('SIGTERM')
}
