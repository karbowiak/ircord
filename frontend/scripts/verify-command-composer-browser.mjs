// Guided slash commands and statuses, using only a disposable demo and no real IRC network.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { spawn } from 'node:child_process'
import { createServer } from 'node:net'
import { mkdir } from 'node:fs/promises'
import { chromium, expect } from 'playwright/test'
import { commands, commandAliases } from '../shared/utils/commands.ts'

const probe = createServer()
await new Promise((resolve) => probe.listen(0, '127.0.0.1', resolve))
const port = probe.address().port
await new Promise((resolve) => probe.close(resolve))
const base = `http://127.0.0.1:${port}`
const server = spawn(process.execPath, ['.output/server/index.mjs'], {
  env: {
    ...process.env,
    HOST: '127.0.0.1',
    PORT: String(port),
    NUXT_DEMO: 'true',
    NUXT_BACKEND_URL: 'http://127.0.0.1:1',
    NUXT_PUBLIC_KLIPY_API_KEY: 'command-fixture',
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
let log = ''
for (const stream of [server.stdout, server.stderr])
  stream.on('data', (chunk) => (log = (log + chunk).slice(-5000)))
let browser
let context
let page
let phase = 'startup'
try {
  await mkdir('.data', { recursive: true })
  for (let attempt = 0; ; attempt++) {
    try {
      if ((await fetch(base)).ok) break
    } catch {
      // The isolated server is still starting.
    }
    assert(attempt < 100 && server.exitCode === null, `Demo did not start: ${log}`)
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  browser = await chromium.launch({ headless: true })
  context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `status${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const bootstrap = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  assert.equal(bootstrap.demo, true)
  const networkId = bootstrap.networks[0].id
  const profilePath = `/api/v1/networks/${encodeURIComponent(networkId)}/profile`
  const selfPath = `/api/v1/networks/${encodeURIComponent(networkId)}/self`
  const profile = async () => (await context.request.get(base + profilePath)).json()
  const requests = []
  const errors = []
  page = await context.newPage()
  page.setDefaultTimeout(8000)
  page.on('pageerror', (error) => errors.push(error.message))
  // The isolated demo retains its WebSocket message transport; production sends over HTTP.
  page.on('websocket', (socket) =>
    socket.on('framesent', ({ payload }) => {
      const body = JSON.parse(String(payload))
      if (body.type === 'message.send')
        requests.push({ path: 'demo:message.send', body, publicCommand: true })
    }),
  )
  const emojiImage = `${base}/media/status-kekw.png`
  await context.route('**/api/v1/emotes', (route) =>
    route.fulfill({
      json: {
        version: 1,
        canManage: false,
        canAdd: true,
        maxBytes: 262144,
        types: ['image/png'],
        providers: [],
        items: [
          {
            id: 'status-kekw',
            name: 'KEKW',
            shortcode: 'kekw',
            aliases: ['KEKW'],
            retired: [],
            enabled: true,
            animated: false,
            url: emojiImage,
            staticUrl: emojiImage,
            width: 1,
            height: 1,
            source: 'upload',
            canManage: false,
            createdAt: '2026-11-14T00:00:00Z',
            updatedAt: '2026-11-14T00:00:00Z',
          },
        ],
      },
    }),
  )
  await context.route('**/media/status-kekw.png', (route) =>
    route.fulfill({
      contentType: 'image/png',
      body: Buffer.from(
        'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jB1sAAAAASUVORK5CYII=',
        'base64',
      ),
    }),
  )
  const gifUrl = 'https://static.klipy.com/command-fixture.gif'
  const gifFile = { url: gifUrl, width: 1, height: 1 }
  await context.route('https://api.klipy.com/**', (route) =>
    route.fulfill({
      headers: { 'access-control-allow-origin': '*' },
      json: {
        result: true,
        data: [
          {
            id: 'command-fixture',
            slug: 'command-fixture',
            title: 'Disposable GIF fixture',
            file: { hd: { gif: gifFile }, sm: { webp: gifFile, jpg: gifFile } },
          },
        ],
      },
    }),
  )
  await context.route(gifUrl, (route) =>
    route.fulfill({
      contentType: 'image/gif',
      body: Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64'),
    }),
  )
  await context.route('**/api/v1/bootstrap', async (route) => {
    const response = await route.fetch()
    const data = await response.json()
    const peer = data.members.find((member) => member.id === 'jules')
    assert(peer, 'The disposable fixture must include the DM partner')
    Object.assign(peer, { statusEmoji: ':kekw:', statusText: 'Working on a project' })
    await route.fulfill({ response, json: data })
  })
  page.on('request', (request) => {
    const path = new URL(request.url()).pathname
    if (request.method() === 'PATCH' && [profilePath, selfPath].includes(path))
      requests.push({ path, body: request.postDataJSON() })
    if (request.method() === 'POST' && /\/(?:messages|commands)$/.test(path))
      requests.push({ path, body: request.postDataJSON(), publicCommand: true })
    if (request.method() === 'POST' && path === '/api/v1/conversations')
      requests.push({ path, body: request.postDataJSON(), conversation: true })
  })
  await page.goto(base)
  await page.waitForURL(`${base}/channel/general`)
  const composer = page.getByRole('textbox', { name: 'Message #general', exact: true })
  const normalHeight = (await page.locator('form.composer').boundingBox()).height
  const expectComposerHeight = async (target, height = normalHeight) => {
    await expect
      .poll(async () => Math.round((await target.boundingBox())?.height ?? 0))
      .toBe(Math.round(height))
  }
  const editor = page.locator('[data-status-editor]')
  const trigger = page.locator('[data-self-menu-trigger]')
  const menu = page.getByRole('menu', { name: 'Open your menu' })
  const openMenu = async () => {
    await trigger.click()
    await expect(menu).toBeVisible()
  }

  // Status stays below the name in the roster, DM list, and fixed-height DM header.
  phase = 'status presentation'
  const memberRow = page.locator('.member-button').filter({ hasText: 'Jules' })
  const directRow = page
    .locator('.conversation-sidebar .conversation-link')
    .filter({ hasText: 'Jules' })
  await expect(memberRow.locator('[data-member-status]')).toContainText('Working on a project')
  await expect(directRow.locator('[data-member-status]')).toContainText('Working on a project')
  await expect(memberRow.locator('[data-member-status] img')).toBeVisible()
  const rowBox = await memberRow.boundingBox()
  assert(rowBox.height <= 40, 'A custom status must fit the normal member row')
  const nameBox = await memberRow.locator('.member-name').boundingBox()
  const statusBox = await memberRow.locator('[data-member-status]').boundingBox()
  assert(statusBox.y >= nameBox.y + nameBox.height - 1, 'The status belongs beneath the name')
  await directRow.click()
  await expect(page.locator('[data-direct-status]')).toContainText('Working on a project')
  await expect(page.locator('[data-direct-status] img')).toBeVisible()
  await page.screenshot({ path: '.data/status-direct-desktop.png' })
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: /^general(?:\s*,|$)/ })
    .click()
  await expect(composer).toBeVisible()

  // Choosing a command enters argument boxes. Tab moves between fields without losing the reply.
  const message = page.locator('article.message-row').last()
  await message.hover()
  await message.getByRole('button', { name: 'Reply to message' }).click()
  await composer.fill('/sta')
  await expect(page.getByText('Set your status', { exact: true })).toBeVisible()
  await composer.press('Tab')
  await expect(editor).toBeVisible()
  await expectComposerHeight(editor)
  await expect(editor.getByLabel('Status emoji', { exact: true })).toBeFocused()
  await expect(page.locator('.composer-reply')).toBeVisible()
  assert.equal(await page.locator('[role="dialog"]').count(), 0)
  assert(await editor.evaluate((element) => !!element.closest('.composer-area')))
  await editor.getByLabel('Status emoji', { exact: true }).fill('☕')
  await editor.getByLabel('Status emoji', { exact: true }).press('Tab')
  await expect(editor.getByRole('textbox', { name: 'Status text' })).toBeFocused()
  await editor.getByRole('textbox', { name: 'Status text' }).fill('In a meeting')
  await editor.getByRole('textbox', { name: 'Status text' }).press('Tab')
  await expect(editor.getByRole('combobox', { name: 'Clear after' })).toBeFocused()
  await editor.getByRole('combobox', { name: 'Clear after' }).selectOption('1h')
  await editor.getByRole('textbox', { name: 'Status text' }).press('Enter')
  await expect(editor).toHaveCount(0)
  const first = await profile()
  assert.equal(first.statusText, 'In a meeting')
  assert.equal(first.statusEmoji, '☕')
  assert(Math.abs(Date.parse(first.statusExpiresAt) - Date.now() - 3_600_000) < 15_000)
  await expect(composer).toBeFocused()
  await expect(page.locator('.composer-reply')).toBeVisible()
  await page.getByRole('button', { name: 'Cancel reply' }).click()

  // Soft-keyboard insertion and command casing enter the same structured flow.
  await composer.fill('/STATUS')
  await page.keyboard.insertText(' ')
  await expect(editor).toBeVisible()
  await editor.getByLabel('Status emoji', { exact: true }).press('Escape')
  await expect(editor).toHaveCount(0)
  await expect(composer).toHaveValue('/STATUS ')

  // The self menu is click-driven, preserves drafts, and can open every requested destination.
  await composer.fill('Unsent draft stays here')
  await openMenu()
  await expect(menu.getByRole('menuitem', { name: 'Edit profile', exact: true })).toBeVisible()
  await expect(menu.getByRole('menuitem', { name: 'Notifications', exact: true })).toBeVisible()
  await expect(menu.getByRole('menuitem', { name: 'Account settings', exact: true })).toBeVisible()
  await page.screenshot({ path: '.data/status-menu-desktop.png' })
  await menu.getByRole('menuitem', { name: '☕ In a meeting', exact: true }).click()
  await expect(editor).toBeVisible()
  await expect(editor.getByLabel('Status emoji', { exact: true })).toBeFocused()
  await editor.getByRole('textbox', { name: 'Status text' }).press('Escape')
  await expect(editor).toHaveCount(0)
  await expect(composer).toBeFocused()
  await expect(composer).toHaveValue('Unsent draft stays here')

  await openMenu()
  await menu.getByRole('menuitem', { name: 'Set yourself away', exact: true }).click()
  await expect.poll(async () => (await profile()).status).toBe('away')
  assert.equal((await profile()).statusText, 'In a meeting')
  assert.deepEqual(requests.filter((r) => r.path === selfPath).at(-1).body, { away: 'Away' })
  await openMenu()
  await menu.getByRole('menuitem', { name: 'Set yourself back', exact: true }).click()
  await expect.poll(async () => (await profile()).status).toBe('online')
  assert.equal((await profile()).statusText, 'In a meeting')

  // Direct status commands never send a message or change away state.
  await composer.fill('/status 🌿 Reading --for 30m')
  await composer.press('Enter')
  await expect(composer).toHaveValue('')
  let current = await profile()
  assert.equal(current.statusText, 'Reading')
  assert.equal(current.statusEmoji, '🌿')
  assert.equal(current.status, 'online')
  await composer.fill('/status Reading --for 0m')
  await composer.press('Enter')
  await expect(composer).toHaveValue('/status Reading --for 0m')
  assert.equal((await profile()).statusText, 'Reading')
  await composer.fill('/status clear')
  await composer.press('Enter')
  await expect(composer).toHaveValue('')
  current = await profile()
  assert(!current.statusText && !current.statusEmoji && !current.statusExpiresAt)

  // A rejected save keeps the panel and editable values, then allows a retry.
  await composer.fill('/status')
  await composer.press('Space')
  await expect(editor).toBeVisible()
  await editor.getByRole('textbox', { name: 'Status text' }).fill('Retry this status')
  await page.route(`**${profilePath}`, (route) =>
    route.request().method() === 'PATCH'
      ? route.fulfill({ status: 503, json: { detail: 'Temporary status failure' } })
      : route.continue(),
  )
  await editor.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(editor.getByRole('alert')).toBeVisible()
  await expectComposerHeight(editor)
  await expect(editor.getByRole('textbox', { name: 'Status text' })).toHaveValue(
    'Retry this status',
  )
  await page.unroute(`**${profilePath}`)
  await editor.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(editor).toHaveCount(0)
  assert.equal((await profile()).statusText, 'Retry this status')

  // Both bare and colon-wrapped names complete against the server's emoji catalog.
  await composer.fill('/status')
  await composer.press('Space')
  const emojiArgument = editor.getByLabel('Status emoji', { exact: true })
  await emojiArgument.fill('kekw')
  await expect(editor.getByRole('option').filter({ hasText: ':kekw:' })).toBeVisible()
  await emojiArgument.press('Tab')
  await expect(editor.getByRole('textbox', { name: 'Status text' })).toBeFocused()
  await expect(emojiArgument).toHaveValue(':kekw:')
  await expect(editor.getByRole('img', { name: ':kekw:', exact: true })).toBeVisible()
  await emojiArgument.fill(':kekw:')
  await expect(editor.getByRole('option').filter({ hasText: ':kekw:' })).toBeVisible()
  await emojiArgument.press('Enter')
  await expect(editor).toBeVisible()
  await expect(emojiArgument).toHaveValue(':kekw:')
  await editor.getByRole('textbox', { name: 'Status text' }).fill('Custom emoji status')
  await editor.getByRole('textbox', { name: 'Status text' }).press('Enter')
  await expect(editor).toHaveCount(0)
  assert.equal((await profile()).statusEmoji, ':kekw:')
  await expect(trigger.getByRole('img', { name: ':kekw:', exact: true })).toBeVisible()
  const selfBox = await trigger.boundingBox()
  assert(selfBox.height <= 44, 'The avatar menu trigger must remain a compact two-line pill')
  await openMenu()
  await menu.getByRole('menuitem', { name: 'View profile', exact: true }).click()
  await expect(page.locator('.profile-status img')).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await openMenu()
  await menu.getByRole('menuitem', { name: 'Clear status', exact: true }).click()
  await expect.poll(async () => (await profile()).statusEmoji ?? '').toBe('')
  await composer.fill('/status Retry this status')
  await composer.press('Enter')
  await expect(composer).toHaveValue('')

  // Pages without a composer use the same editor anchored to the avatar menu.
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: 'Search messages' })
    .click()
  await page.waitForURL(`${base}/search`)
  await expect(composer).toHaveCount(0)
  await openMenu()
  await menu.getByRole('menuitem', { name: 'Retry this status', exact: true }).click()
  await expect(editor).toBeVisible()
  await expect(editor.getByLabel('Status emoji', { exact: true })).toBeFocused()
  await editor.getByRole('textbox', { name: 'Status text' }).press('Escape')
  await expect(editor).toHaveCount(0)
  await expect(trigger).toBeFocused()

  // The menu and editor remain inside a phone viewport and do not create a dialog.
  const phone = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
    serviceWorkers: 'block',
  })
  await phone.addCookies(await context.cookies())
  const phonePage = await phone.newPage()
  await phonePage.goto(`${base}/channel/general`)
  const phoneHeight = (await phonePage.locator('form.composer').boundingBox()).height
  await phonePage.locator('[data-self-menu-trigger]').click()
  await phonePage.getByRole('menuitem', { name: 'Retry this status', exact: true }).click()
  const phoneEditor = phonePage.locator('[data-status-editor]')
  await expect(phoneEditor).toBeVisible()
  await expectComposerHeight(phoneEditor, phoneHeight)
  await phoneEditor.getByRole('textbox', { name: 'Status text' }).focus()
  await expect(phoneEditor.getByRole('textbox', { name: 'Status text' })).toBeFocused()
  const statusTextBox = await phoneEditor
    .getByRole('textbox', { name: 'Status text' })
    .boundingBox()
  assert(statusTextBox.x >= 0 && statusTextBox.x + statusTextBox.width <= 391)
  await phonePage.screenshot({ path: '.data/status-editor-mobile.png' })
  const phoneBox = await phoneEditor.boundingBox()
  assert(
    phoneBox.x >= 0 && phoneBox.x + phoneBox.width <= 391,
    'Phone editor overflows the viewport',
  )
  assert.equal(await phonePage.locator('[role="dialog"]').count(), 0)
  await phone.close()
  assert.equal(
    requests.some((r) => r.publicCommand),
    false,
    'Status actions sent an IRC message',
  )

  // Other commands reuse the same argument row and autocomplete providers.
  phase = 'guided private message'
  await page.goto(`${base}/channel/general`)
  await expect(composer).toBeVisible()
  const commandEditor = page.locator('[data-command-editor]')
  await composer.fill('/msg')
  await composer.press('Space')
  await expect(commandEditor).toHaveAttribute('data-command-name', 'msg')
  await expectComposerHeight(commandEditor)
  const nickname = commandEditor.locator('[data-command-argument="nickname"]')
  const messageField = commandEditor.locator('[data-command-argument="message"]')
  await expect(nickname).toBeFocused()
  await nickname.fill('ju')
  await expect(
    commandEditor.getByRole('option').filter({ hasText: /jules/i }).first(),
  ).toBeVisible()
  await nickname.press('Tab')
  await expect(nickname).toHaveValue('jules')
  await expect(messageField).toBeFocused()
  await messageField.fill('A guided  message with  spaces')
  await page.screenshot({ path: '.data/command-msg-desktop.png' })
  await messageField.press('Enter')
  await expect(commandEditor).toHaveCount(0)
  await expect
    .poll(() => requests.filter((request) => request.publicCommand).at(-1)?.body.text)
    .toBe('A guided  message with  spaces')
  await page.waitForURL(`${base}/dm/**`)

  // Optional arguments, channel completion, masked keys, failure recovery, and history privacy.
  phase = 'guided channel join'
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: /^general(?:\s*,|$)/ })
    .click()
  await composer.fill('/join')
  await composer.press('Space')
  await expect(commandEditor).toHaveAttribute('data-command-name', 'join')
  const channel = commandEditor.locator('[data-command-argument="channel"]')
  const key = commandEditor.locator('[data-command-argument="key"]')
  await expect(channel).toBeFocused()
  await channel.fill('des')
  await expect(
    commandEditor.getByRole('option').filter({ hasText: '#design' }).first(),
  ).toBeVisible()
  await channel.press('Tab')
  await expect(channel).toHaveValue('#design')
  await expect(key).toBeFocused()
  await expect(key).toHaveAttribute('type', 'password')
  const disposableKey = 'fixture-only-key'
  await key.fill(disposableKey)
  const browserState = await page.evaluate(() =>
    JSON.stringify({
      history: history.state,
      local: { ...localStorage },
      session: { ...sessionStorage },
      url: location.href,
    }),
  )
  assert(!browserState.includes(disposableKey), 'Command secrets entered browser persistence')
  await page.route('**/api/v1/conversations', (route) =>
    route.request().method() === 'POST'
      ? route.fulfill({ status: 503, json: { detail: 'Temporary join failure' } })
      : route.continue(),
  )
  await key.press('Enter')
  await expect(commandEditor.getByRole('alert')).toBeVisible()
  await expectComposerHeight(commandEditor)
  await expect(key).toHaveValue(disposableKey)
  await page.unroute('**/api/v1/conversations')
  await key.press('Enter')
  await expect(commandEditor).toHaveCount(0)
  assert.deepEqual(requests.filter((request) => request.conversation).at(-1)?.body, {
    networkId,
    name: '#design',
    key: disposableKey,
  })

  // A free-text command preserves internal spaces and runs the existing handler.
  phase = 'guided action'
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: /^general(?:\s*,|$)/ })
    .click()
  await composer.fill('/me')
  await composer.press('Space')
  await expect(commandEditor).toHaveAttribute('data-command-name', 'me')
  await commandEditor.locator('[data-command-argument="action"]').fill('waves  at everyone')
  await commandEditor.locator('[data-command-argument="action"]').press('Enter')
  await expect(commandEditor).toHaveCount(0)
  await expect
    .poll(() => requests.filter((request) => request.publicCommand).at(-1)?.body.kind)
    .toBe('action')
  assert.equal(
    requests.filter((request) => request.publicCommand).at(-1)?.body.text,
    'waves  at everyone',
  )

  // Guided GIFs retain the reply target and clear only the reply they used after sending.
  phase = 'guided GIF reply'
  const repliedMessage = page.locator('article.message-row').last()
  await repliedMessage.hover()
  await repliedMessage.getByRole('button', { name: 'Reply to message' }).click()
  await composer.fill('/gif')
  await composer.press('Space')
  await commandEditor.locator('[data-command-argument="query"]').fill('disposable fixture')
  await commandEditor.locator('[data-command-argument="query"]').press('Enter')
  await expect(commandEditor).toHaveCount(0)
  await expect
    .poll(() => requests.filter((request) => request.publicCommand).at(-1)?.body.gif?.slug)
    .toBe('command-fixture')
  const gifRequest = requests.filter((request) => request.publicCommand).at(-1)
  assert.equal(gifRequest.body.conversationId, 'general')
  assert(gifRequest.body.replyToId, 'Guided GIFs must include the captured reply')
  await expect(page.locator('.composer-reply')).toHaveCount(0)

  // No-argument commands keep their direct execution path; Escape cancels guided fields.
  await composer.fill('/back')
  await composer.press('Enter')
  await expect(composer).toHaveValue('')
  await expect(commandEditor).toHaveCount(0)
  await composer.fill('/msg')
  await composer.press('Space')
  await expect(commandEditor).toBeVisible()
  await commandEditor.locator('[data-command-argument="nickname"]').press('Escape')
  await expect(commandEditor).toHaveCount(0)
  await expect(composer).toHaveValue('/msg')
  await expect(composer).toBeFocused()

  // New nicknames are free entry, and rejected commands keep their editable fields.
  await composer.fill('/nick')
  await composer.press('Space')
  const newNickname = commandEditor.locator('[data-command-argument="nickname"]')
  await expect(newNickname).toHaveAttribute('type', 'text')
  await newNickname.fill('123invalid')
  await expect(commandEditor.getByRole('option')).toHaveCount(0)
  await newNickname.press('Enter')
  await expect(commandEditor.getByRole('alert')).toBeVisible()
  await expect(newNickname).toHaveValue('123invalid')
  await newNickname.press('Escape')
  await expect(commandEditor).toHaveCount(0)
  await expect(composer).toHaveValue('/nick')

  // A slow DM request keeps its target and does not pull the user out of another channel.
  phase = 'delayed private message'
  let releaseDirect
  await page.route('**/api/v1/conversations', async (route) => {
    if (route.request().method() !== 'POST') return route.continue()
    const response = await route.fetch()
    await new Promise((resolve) => (releaseDirect = resolve))
    await route.fulfill({ response })
  })
  await composer.fill('/msg')
  await composer.press('Space')
  await commandEditor.locator('[data-command-argument="nickname"]').fill('jules')
  await commandEditor.locator('[data-command-argument="message"]').fill('Delayed private message')
  await commandEditor.locator('[data-command-argument="message"]').press('Enter')
  await expect.poll(() => typeof releaseDirect).toBe('function')
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: /^design(?:\s*,|$)/ })
    .click()
  await page.waitForURL(`${base}/channel/design`)
  releaseDirect()
  await expect
    .poll(() => requests.filter((request) => request.publicCommand).at(-1)?.body.text)
    .toBe('Delayed private message')
  const privateRequest = requests.filter((request) => request.publicCommand).at(-1)
  assert.equal(privateRequest.body.conversationId, 'dm-jules')
  await expect(page.getByRole('textbox', { name: 'Message #design', exact: true })).toBeVisible()
  assert.equal(page.url(), `${base}/channel/design`)
  await page.unroute('**/api/v1/conversations')
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: /^general(?:\s*,|$)/ })
    .click()

  // An optional list filter reaches the channel directory instead of being discarded.
  phase = 'channel directory filter'
  await composer.fill('/list')
  await composer.press('Space')
  await commandEditor.locator('[data-command-argument="filter"]').fill('design')
  await commandEditor.locator('[data-command-argument="filter"]').press('Enter')
  const joinDialog = page.getByRole('dialog', { name: 'Join a channel', exact: true })
  await expect(joinDialog).toBeVisible()
  await expect(joinDialog.locator('.join-form input')).toHaveValue('design')
  await joinDialog.getByRole('button', { name: 'Close dialog' }).click()

  // Every parameterized catalog entry renders through the shared editor without executing it.
  phase = 'command catalog and aliases'
  for (const spec of commands.filter(
    (command) => command.fields.length && command.name !== 'status',
  )) {
    await composer.fill(`/${spec.name}`)
    await composer.press('Space')
    await expect(commandEditor).toHaveAttribute('data-command-name', spec.name)
    for (const field of spec.fields)
      await expect(commandEditor.locator(`[data-command-argument="${field.name}"]`)).toBeVisible()
    await page.keyboard.press('Escape')
    await page.keyboard.press('Escape')
    await expect(commandEditor).toHaveCount(0)
    await expect(composer).toHaveValue(`/${spec.name}`)
  }
  for (const [alias, canonical] of Object.entries(commandAliases)) {
    await composer.fill(`/${alias}`)
    await composer.press('Space')
    await expect(commandEditor).toHaveAttribute('data-command-name', canonical)
    await page.keyboard.press('Escape')
    await page.keyboard.press('Escape')
    await expect(commandEditor).toHaveCount(0)
  }
  // The production demo also starts the unrelated offline SQLite worker. Its current CSP
  // blocks WebAssembly compilation; retain that finding without masking other page errors.
  const offlineCsp = errors.filter(
    (error) =>
      error.includes('WebAssembly.instantiate()') && error.includes('Content Security policy'),
  )
  assert.deepEqual(
    errors.filter((error) => !offlineCsp.includes(error)),
    [],
  )
  if (offlineCsp.length)
    console.warn('Separate offline-worker CSP errors observed:', offlineCsp.length)
  console.log(
    'Command browser checks passed: typed arguments, nickname/channel/server-emoji completion, secrets, retry, status/menu/away, drafts/replies, and phone layout.',
  )
} catch (error) {
  console.error(`Command browser check failed during ${phase}: ${page?.url() ?? base}`)
  if (page && !page.isClosed())
    await page.screenshot({ path: '.data/command-failure.png' }).catch(() => {})
  throw error
} finally {
  if (context) await context.close()
  if (browser) await browser.close()
  server.kill('SIGTERM')
}
