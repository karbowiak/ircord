// A disposable production-server check. The existing frontend dev server is untouched.
import { spawn } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import { createServer } from 'node:net'
import { once } from 'node:events'
import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'
import WebSocket from 'ws'
import validateEvent from '../shared/generated/event-validator.js'

const ircHost = process.env.IRCORD_TEST_IRC_HOST || '127.0.0.1'
const ircTLS = process.env.IRCORD_TEST_IRC_TLS === 'true'
const ircPort = Number(process.env.IRCORD_TEST_IRC_PORT || (ircTLS ? 6697 : 16667))
const ircPassword = process.env.IRCORD_TEST_SERVER_PASSWORD_FILE
  ? (await readFile(process.env.IRCORD_TEST_SERVER_PASSWORD_FILE, 'utf8')).trim()
  : ''
const redact = (text) => {
  const value = String(text).replace(/ircord-[A-Z0-9-]{20,}/gi, '[test key redacted]')
  return ircPassword ? value.split(ircPassword).join('[server password redacted]') : value
}

const freePort = () =>
  new Promise((done, reject) => {
    const server = createServer().listen(0, '127.0.0.1', () => {
      const port = server.address().port
      server.close(() => done(port))
    })
    server.on('error', reject)
  })
const delay = (ms) => new Promise((done) => setTimeout(done, ms))
const until = async (check, label) => {
  for (let i = 0; i < 200; i++) {
    if (await check()) return
    await delay(100)
  }
  throw new Error(`Timed out: ${label}`)
}
const refusedSocket = (url, origin, status) => new Promise((resolve, reject) => {
  const socket = new WebSocket(url.replace(/^http/, 'ws'), { headers: origin ? { Origin: origin } : {}, handshakeTimeout: 5000 })
  let answered = false
  const timer = setTimeout(() => { socket.terminate(); reject(new Error('Rejected socket handshake timed out')) }, 6000)
  socket.once('open', () => { socket.close(); reject(new Error('Unauthenticated or foreign-origin socket was accepted')) })
  socket.once('unexpected-response', (_request, response) => {
    answered = true; clearTimeout(timer)
    response.resume(); socket.terminate()
    try { assert.equal(response.statusCode, status); resolve() } catch (error) { reject(error) }
  })
  socket.once('error', error => { clearTimeout(timer); if (!answered) reject(error) })
})
const external = process.env.IRCORD_TEST_PUBLIC_URL?.replace(/\/$/, '')
const createdKeys = []
const dir = await mkdtemp(resolve(tmpdir(), 'ircord-browser-'))
const apiPort = await freePort(),
  uiPort = await freePort()
const api = external || `http://127.0.0.1:${apiPort}`,
  ui = external || `http://127.0.0.1:${uiPort}`
let apiLog = '',
  uiLog = ''
const backend = external ? null : spawn(resolve('../.data/bin/ircord'), [], {
  env: {
    ...process.env,
    IRCORD_DATA_DIR: resolve(dir, 'backend'),
    IRCORD_LISTEN: `127.0.0.1:${apiPort}`,
    IRCORD_PUBLIC_URL: ui,
    IRCORD_ALLOWED_ORIGINS: ui,
    IRCORD_ALLOW_PRIVATE_IRC: 'true',
    IRCORD_NETWORK_PASSWORDS: ircTLS && ircPassword ? JSON.stringify({ [`${ircHost}:${ircPort}`]: ircPassword }) : '',
    IRCORD_NETWORK_PASSWORDS_FILE: '',
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
backend?.stdout.on('data', (v) => (apiLog += v))
backend?.stderr.on('data', (v) => (apiLog += v))
const frontend = external ? null : spawn(process.execPath, ['.output/server/index.mjs'], {
  env: {
    ...process.env,
    HOST: '127.0.0.1',
    PORT: String(uiPort),
    NUXT_PUBLIC_API_BASE: '/api/v1',
    NUXT_DEMO: 'false',
    NUXT_BACKEND_URL: api,
    NUXT_PUBLIC_KLIPY_API_KEY: 'test-app-key',
    NUXT_PUBLIC_NETWORK_PRESETS: JSON.stringify([{ name: 'IRCord', host: ircHost, port: ircPort, tls: ircTLS, authMethod: ircPassword ? 'server-password' : 'none', autoJoin: ['#general'], passwordProvided: !!ircPassword && ircTLS }]),
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
frontend?.stdout.on('data', (v) => (uiLog += v))
frontend?.stderr.on('data', (v) => (uiLog += v))
let browser, page
const requests = []
try {
  await until(async () => {
    try {
      return (await fetch(`${api}/healthz`)).ok && (await fetch(ui)).ok
    } catch {
      return false
    }
  }, 'isolated servers')
  await refusedSocket(`${ui}/api/v1/events`, ui, 401)
  await refusedSocket(`${ui}/api/v1/events`, 'https://foreign.invalid', 403)
  await refusedSocket(`${ui}/api/v1/events`, '', external ? 401 : 403)
  browser = await chromium.launch({ headless: true })
  const context = await browser.newContext()
  page = await context.newPage()
  page.on('requestfailed', (r) => requests.push(`${r.failure()?.errorText} ${r.url()}`))
  page.on('response', (r) => {
    if (r.url().includes('/api/')) requests.push(`${r.status()} ${r.url()}`)
  })
  const errors = []
  page.on('pageerror', (e) => errors.push(e.message))
  const frames = []
  const observe = target => target.on('websocket', socket => socket.on('framereceived', frame => {
    try {
      const event = JSON.parse(String(frame.payload))
      if (!validateEvent(event)) errors.push(`Live event violates Go schema: ${event.type}`)
      frames.push(event)
    } catch { errors.push('Invalid live event JSON') }
  }))
  observe(page)
  await page.route('https://api.klipy.com/**', (route) => route.abort())
  await page.goto(ui)
  await page.waitForFunction(() => document.querySelector('h1'))
  if ((await page.getByRole('button', { name: 'Create an account', exact: true }).count()) === 0) {
    const state = await page.evaluate(() => {
      const app = document.querySelector('#__nuxt')?.__vue_app__
      const store = app?.config.globalProperties.$pinia?._s.get('auth')
      const error = store?.session?.error
      return {
        message: error?.message ?? error?.value?.message,
        stack: error?.stack ?? error?.value?.stack,
        apiBase: window.__NUXT__?.config?.public?.apiBase,
      }
    })
    throw new Error(JSON.stringify(state))
  }
  const registration = page.waitForResponse(response => response.url().endsWith('/api/v1/auth/register') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Create an account', exact: true }).click()
  createdKeys.push((await (await registration).json()).key)
  await page.getByLabel('I’ve saved my key somewhere safe').check()
  await page.getByRole('button', { name: 'Continue to IRCord', exact: true }).click()
  await until(
    async () => (await context.request.get(`${ui}/api/v1/bootstrap`)).ok(),
    'authenticated workspace',
  )
  const suffix = Date.now().toString(36)
  const room = `#browser${suffix}`
  const guide = page.getByRole('dialog', { name: 'Your first connection', exact: true })
  await guide.waitFor({ state: 'visible' })
  assert.equal(await guide.getByLabel('Nickname', { exact: true }).inputValue(), '', 'nickname is chosen by the new user')
  assert.equal(await page.evaluate(() => document.activeElement?.id), 'setup-nick', 'nickname has initial focus')
  assert.equal(await guide.getByRole('button', { name: 'Connection settings', exact: true }).getAttribute('aria-expanded'), 'false')
  await guide.getByRole('button', { name: 'Set up later', exact: true }).click()
  await page.reload()
  await page.getByRole('heading', { name: 'Add your first network', exact: true }).waitFor()
  assert.equal(await guide.count(), 0, 'dismissed onboarding does not reopen after reload')
  await page.getByRole('button', { name: 'Add a network', exact: true }).first().click()
  await guide.waitFor({ state: 'visible' })
  await guide.getByRole('button', { name: 'Connection settings', exact: true }).click()
  assert.equal(await page.getByLabel('Hostname', { exact: true }).inputValue(), ircHost, 'environment preset fills the server')
  assert.equal(await page.getByLabel('Port', { exact: true }).inputValue(), String(ircPort))
  assert.equal(await page.getByLabel('Join these channels', { exact: true }).inputValue(), '#general')
  assert.equal(await page.getByLabel('Password', { exact: true }).count(), ircPassword ? 1 : 0)
  await guide.getByLabel('Nickname', { exact: true }).fill(`web${suffix}`)
  await guide.getByLabel('Name', { exact: true }).fill('')
  await guide.getByRole('button', { name: 'Connection settings', exact: true }).click()
  await guide.getByRole('button', { name: 'Connect', exact: true }).click()
  await page.waitForFunction(() => document.activeElement?.id === 'network-name')
  assert.equal(await guide.getByRole('button', { name: 'Connection settings', exact: true }).getAttribute('aria-expanded'), 'true', 'invalid hidden settings are revealed and focused')
  await page.getByLabel('Name', { exact: true }).fill('Browser Ergo')
  await page.getByLabel('Hostname', { exact: true }).fill(ircHost)
  await page.getByLabel('Port', { exact: true }).fill(String(ircPort))
  await page.getByLabel('TLS', { exact: true }).setChecked(ircTLS)
  await page.getByLabel('Nickname', { exact: true }).fill(`web${suffix}`)
  if (ircPassword) {
    await page.getByRole('radio', { name: 'Server password', exact: true }).click()
    if (!ircTLS) await page.getByLabel('Password', { exact: true }).fill(ircPassword)
    else assert.equal(await page.getByLabel('Password', { exact: true }).getAttribute('required'), null, 'private server default avoids entering a password')
  }
  await page.getByLabel('Join these channels', { exact: true }).fill(room)
  const failConnection = route => route.fulfill({ status: 503, contentType: 'application/problem+json', body: JSON.stringify({ status: 503, title: 'Unavailable', detail: 'Temporary connection setup failure. Try again.' }) })
  await page.route('**/api/v1/networks', failConnection)
  await guide.getByRole('button', { name: 'Connect', exact: true }).click()
  await guide.getByRole('alert').filter({ hasText: 'Temporary connection setup failure.' }).waitFor()
  assert.equal(await guide.getByLabel('Nickname', { exact: true }).inputValue(), `web${suffix}`, 'failed setup retains the nickname')
  await page.unroute('**/api/v1/networks', failConnection)
  const created = page.waitForResponse(response => response.url().endsWith('/api/v1/networks') && response.request().method() === 'POST')
  await guide.getByRole('button', { name: 'Connect', exact: true }).click()
  const response = await created
  assert.equal(response.status(), 200, 'create real IRC connection')
  const network = await response.json()
  if (ircPassword) assert(!JSON.stringify(network).includes(ircPassword), 'password stays out of the network response')
  const composer = page.getByRole('combobox', { name: `Message ${room}`, exact: true })
  await composer.waitFor({ state: 'visible' })
  await until(() => composer.isEnabled(), 'real IRC channel joined')
  await until(async () => await page.locator('.self-profile strong').innerText() === `web${suffix}`, 'self label uses chosen nickname')
  assert.equal((await (await context.request.get(`${ui}/api/v1/bootstrap`)).json()).user.nick, `web${suffix}`, 'canonical self nick is current')
  await context.request.patch(`${ui}/api/v1/networks/${network.id}/self`, { data: { nick: `web${suffix}x` } })
  await until(async () => await page.locator('.self-profile strong').innerText() === `web${suffix}x`, 'self label follows nick changes')
  await context.request.patch(`${ui}/api/v1/networks/${network.id}/self`, { data: { nick: `web${suffix}` } })
  await until(async () => await page.locator('.self-profile strong').innerText() === `web${suffix}`, 'self nickname restored')
  const peerContext = await browser.newContext()
  createdKeys.push((await (await peerContext.request.post(`${ui}/api/v1/auth/register`)).json()).key)
  const peerNetwork = await peerContext.request.post(`${ui}/api/v1/networks`, { data: {
    name: 'Peer Ergo', host: ircHost, port: ircPort, tls: ircTLS, nick: `peer${suffix}`,
    auth: ircPassword ? { method: 'server-password', password: ircPassword } : { method: 'none' }, autoJoin: [room], perform: [],
  } })
  assert(peerNetwork.ok(), 'second account creates its own connection')
  const peer = await peerContext.newPage()
  observe(peer)
  peer.on('pageerror', error => errors.push(error.message))
  await peer.goto(ui)
  await until(() => peer.getByRole('combobox', { name: `Message ${room}`, exact: true }).isEnabled(), 'peer joined')
  await until(async () => (await (await peerContext.request.get(`${ui}/api/v1/bootstrap`)).json()).members.some(member => member.nick === `web${suffix}`), 'peer knows typing sender')
  await composer.fill('draft ')
  await page.locator('.timeline-scroll').evaluate(element => { element.focus(); window.getSelection()?.removeAllRanges() })
  await page.keyboard.type('hello')
  assert.equal(await composer.inputValue(), 'draft hello', 'type-to-compose preserves the first character')
  await composer.fill('Go transport browser message 👋')
  await peer.locator('.typing-line').filter({ hasText: `web${suffix} is typing` }).waitFor({ state: 'visible' })
  await composer.press('Enter')
  await peer.locator('.typing-line').waitFor({ state: 'hidden' })
  assert(frames.some(event => event.type === 'typing' && !('seq' in event)), 'typing is private and ephemeral')
  await until(
    async () =>
      (await page
        .locator('.message-text')
        .filter({ hasText: /^Go transport browser message 👋$/ })
        .count()) === 1 && (await page.locator('.is-pending').count()) === 0,
    'HTTP send acknowledged over WebSocket',
  )
  const bootstrap = await (await context.request.get(`${ui}/api/v1/bootstrap`)).json()
  const conv = bootstrap.conversations.find((c) => c.networkId === network.id)
  const history = await (
    await context.request.get(`${ui}/api/v1/conversations/${conv.id}/messages`)
  ).json()
  const message = history.items.find((m) => m.text === 'Go transport browser message 👋')
  assert(message?.upstreamId, 'canonical message has an Ergo ID')
  assert.equal(message.senderId, bootstrap.user.id, 'backend keeps actual account identity')
  assert.equal(await page.locator('.message-row').filter({ has: page.locator('.message-text').filter({ hasText: /^Go transport browser message 👋$/ }) }).locator('.author-name').innerText(), `web${suffix}`, 'message author uses nickname')
  assert.equal(bootstrap.demo, false, 'production Go workspace is active')
  // Replies use an HTTP mutation and retain canonical upstream IDs.
  const original = page.locator('.message-row').filter({ has: page.locator('.message-text').filter({ hasText: /^Go transport browser message 👋$/ }) })
  await original.hover()
  await original.getByRole('button', { name: 'Reply to message', exact: true }).click()
  await composer.fill('A real reply')
  await composer.press('Enter')
  await until(async () => (await (await context.request.get(`${ui}/api/v1/conversations/${conv.id}/messages`)).json()).items.some(item => item.text === 'A real reply' && item.replyTo?.id === message.id), 'canonical reply')
  assert(
    (
      await context.request.post(`${ui}/api/v1/conversations/${conv.id}/reactions`, {
        data: { messageId: message.id, emoji: '👋' },
      })
    ).ok(),
  )
  const png = Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aGNcAAAAASUVORK5CYII=',
    'base64',
  )
  await page
    .locator('input[type=file]')
    .setInputFiles({ name: 'browser.png', mimeType: 'image/png', buffer: png })
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await until(async () => {
    const page = await (
      await context.request.get(`${ui}/api/v1/conversations/${conv.id}/messages`)
    ).json()
    return page.items.some((m) => m.attachment?.name === 'browser.png')
  }, 'attachment upload and IRC send')
  const fileHistory = await (await context.request.get(`${ui}/api/v1/conversations/${conv.id}/messages`)).json()
  const attachment = fileHistory.items.find(item => item.attachment?.name === 'browser.png').attachment
  assert.equal(new URL(attachment.url).origin, ui, 'files use the frontend origin')
  const file = await peerContext.request.get(attachment.url, { headers: { Range: 'bytes=0-7' } })
  assert.equal(file.status(), 206, 'shared download proxy preserves ranges')
  assert.deepEqual(await file.body(), png.subarray(0, 8))
  const connection = await (await context.request.get(`${ui}/api/v1/networks/${network.id}/connection`)).json()
  assert(connection.negotiatedCapabilities.includes('draft/chathistory'), 'real negotiated capabilities are available')
  const cookies = (await context.cookies(ui)).map(cookie => `${cookie.name}=${cookie.value}`).join('; ')
  await new Promise((resolve, reject) => {
    const socket = new WebSocket(`${ui.replace('http:', 'ws:')}/api/v1/networks/${network.id}/connection/trace/events`, { headers: { Cookie: cookies, Origin: ui } })
    const timer = setTimeout(() => { socket.terminate(); reject(new Error('Diagnostic proxy timed out')) }, 5000)
    socket.once('error', reject)
    socket.once('message', raw => {
      clearTimeout(timer); socket.close()
      try { assert.equal(JSON.parse(String(raw)).type, 'connection.line'); resolve() } catch (error) { reject(error) }
    })
  })
  const library = await context.request.patch(`${ui}/api/app/library`, {
    data: { uses: { '1F44B': 2 }, skinTone: '2' },
  })
  assert.equal(library.status(), 200, 'Nitro forwards the Go session and saves app data')
  const document = await (await context.request.get(`${ui}/api/v1/app-data/library`)).json()
  assert.equal(document.data.skinTone, '2')
  assert.equal(document.data.emojiUsage[0].count, 2)
  const invitedRoom = `#invite${suffix}`
  const invited = await context.request.post(`${ui}/api/v1/conversations`, { data: { networkId: network.id, name: invitedRoom } })
  assert(invited.ok())
  await until(async () => (await (await context.request.get(`${ui}/api/v1/bootstrap`)).json()).conversations.some(conv => conv.target === invitedRoom && conv.joined), 'inviter joins private test channel')
  assert((await context.request.post(`${ui}/api/v1/networks/${network.id}/commands`, { data: { clientId: `invite-${suffix}`, line: `INVITE peer${suffix} ${invitedRoom}` } })).ok())
  await until(async () => (await (await peerContext.request.get(`${ui}/api/v1/bootstrap`)).json()).invitations.some(invite => invite.channel === invitedRoom), 'invite reaches the private workspace')
  const inviteBanner = peer.getByRole('status', { name: 'Channel invitation' })
  await inviteBanner.waitFor()
  assert(await inviteBanner.getByText(invitedRoom, { exact: true }).isVisible())
  assert.equal(await peer.locator('.conversation-link').filter({ hasText: `invite${suffix}` }).count(), 0, 'pending invitation does not create a channel')
  const inviteDesktop = new URL('../.impeccable/review/invitation-desktop.png', import.meta.url)
  await peer.screenshot({ path: inviteDesktop.pathname })
  await peer.setViewportSize({ width: 390, height: 844 })
  await peer.screenshot({ path: new URL('../.impeccable/review/invitation-mobile.png', import.meta.url).pathname })
  await peer.setViewportSize({ width: 1280, height: 900 })
  // Rejection blocks repeats without creating or selecting a conversation.
  await inviteBanner.getByRole('button', { name: 'Reject', exact: true }).click()
  await until(async () => await inviteBanner.count() === 0, 'invitation rejected')
  assert((await context.request.post(`${ui}/api/v1/networks/${network.id}/commands`, { data: { clientId: `repeat-${suffix}`, line: `INVITE peer${suffix} ${invitedRoom}` } })).ok())
  await peer.reload()
  await until(async () => (await (await peerContext.request.get(`${ui}/api/v1/bootstrap`)).json()).invitations.length === 0, 'rejection survives reload and repeat invite')
  assert.equal(await peer.locator('.conversation-link').filter({ hasText: `invite${suffix}` }).count(), 0)
  // A different channel can still be accepted explicitly.
  const acceptedRoom = `#accepted${suffix}`
  assert((await context.request.post(`${ui}/api/v1/conversations`, { data: { networkId: network.id, name: acceptedRoom } })).ok())
  await until(async () => (await (await context.request.get(`${ui}/api/v1/bootstrap`)).json()).conversations.some(c => c.target === acceptedRoom && c.joined), 'accepted invitation channel joined')
  assert((await context.request.post(`${ui}/api/v1/networks/${network.id}/commands`, { data: { clientId: `accepted-${suffix}`, line: `INVITE peer${suffix} ${acceptedRoom}` } })).ok())
  await inviteBanner.getByText(acceptedRoom, { exact: true }).waitFor()
  await inviteBanner.getByRole('button', { name: 'Join', exact: true }).click()
  await until(() => peer.getByRole('combobox', { name: `Message ${acceptedRoom}`, exact: true }).isEnabled(), 'invitation accepted explicitly')
  await peer.locator('.conversation-link').filter({ hasText: `browser${suffix}` }).click()
  const profile = await context.request.patch(`${ui}/api/v1/profile`, { data: { displayName: 'Browser Profile', bio: 'Live profile', nameColor: '#ee88cc', avatarDataUrl: `data:image/png;base64,${png.toString('base64')}` } })
  assert.equal(profile.status(), 200, 'profile saves through Go')
  await page.locator('.self-profile .avatar-image').waitFor({ state: 'visible' })
  assert.equal((await (await peerContext.request.get(`${ui}/api/v1/conversations/${conv.id}/messages`)).json()).status, 404, 'conversation IDs remain account scoped')
  // The browser disappears while Go stays on IRC. Reconnection must replay the
  // peer's message exactly once, without requiring a manual reload.
  await context.setOffline(true)
  const peerComposer = peer.getByRole('combobox', { name: `Message ${room}`, exact: true })
  await peerComposer.fill('Message while browser offline')
  await peerComposer.press('Enter')
  await until(async () => (await peer.locator('.message-text').filter({ hasText: /^Message while browser offline$/ }).count()) === 1 && (await peer.locator('.is-pending').count()) === 0, 'peer send while first browser is offline')
  await context.setOffline(false)
  await until(async () => (await page.locator('.message-text').filter({ hasText: /^Message while browser offline$/ }).count()) === 1, 'live reconnect replay without duplicates')
  await page.reload()
  await composer.waitFor({ state: 'visible' })
  await until(
    async () =>
      (await page
        .locator('.message-text')
        .filter({ hasText: /^Go transport browser message 👋$/ })
        .count()) === 1,
    'history after reload',
  )
  assert.deepEqual(errors, [], 'no browser runtime errors')
  assert(!await page.getByText('An update could not be read.', { exact: false }).count(), 'all live events parsed')
  const docs = await context.newPage()
  await docs.goto(`${api}/docs`)
  await docs.locator('.scalar-api-reference').waitFor({ state: 'visible', timeout: 30_000 })
  assert((await docs.locator('body').innerText()).includes('IRCord'), 'Scalar loaded the live OpenAPI')
  assert.deepEqual(await (await context.request.get(`${api}/openapi.json`)).json(), JSON.parse(await readFile('../backend/openapi.json', 'utf8')), 'served and generated OpenAPI match')
  await docs.close()
  await context.request.post(`${ui}/api/v1/auth/logout`)
  assert.equal((await context.request.get(`${ui}/api/v1/bootstrap`)).status(), 401)
  assert.equal((await peerContext.request.get(`${ui}/api/v1/bootstrap`)).status(), 200, 'logout does not revoke another account')
  await page
    .getByRole('button', { name: 'Create an account', exact: true })
    .waitFor({ state: 'visible' })
  console.log(
    'PASS: same-origin Go/Ergo network form, WS auth/origin isolation, typed live events, two-account typing, messages/replies/reactions, profiles, uploads/ranges, diagnostics, app data, offline replay/reload history, Scalar/OpenAPI, and session revocation',
  )
} catch (error) {
  console.error(redact(error.message))
  console.error(redact(requests.slice(-10).join('\n')))
  if (page) console.error(redact(await page.locator('body').innerText()).slice(0, 1600))
  console.error(redact(apiLog.slice(-1500)), redact(uiLog.slice(-1500)))
  process.exitCode = 1
} finally {
  if (external) {
    for (const key of createdKeys) {
      const login = await fetch(`${ui}/api/v1/auth/login`, { method: 'POST', headers: { 'Content-Type': 'application/json', Origin: ui }, body: JSON.stringify({ key }) })
      const cookie = login.headers.get('set-cookie')?.split(';')[0]
      if (!login.ok || !cookie) { console.error('Failed to authenticate disposable account cleanup'); process.exitCode = 1; continue }
      const deleted = await fetch(`${ui}/api/v1/auth/account`, { method: 'DELETE', headers: { Cookie: cookie, Origin: ui } })
      if (!deleted.ok) { console.error('Failed to delete disposable test account'); process.exitCode = 1 }
    }
  }
  await browser?.close()
  const stop = async (child) => {
    if (!child) return
    if (child.exitCode !== null || child.signalCode !== null) return
    const exited = once(child, 'exit')
    child.kill('SIGTERM')
    await Promise.race([exited, delay(5000)])
    if (child.exitCode === null) {
      child.kill('SIGKILL')
      await exited
    }
  }
  await Promise.all([stop(frontend), stop(backend)])
  await rm(dir, { recursive: true, force: true })
}
