// Uses a disposable demo account and intercepts creation; no real IRC server is contacted.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { mkdir } from 'node:fs/promises'
import { chromium } from 'playwright'

const base = process.env.IRCORD_CHANNEL_BASE ?? 'http://127.0.0.1:4175'
const url = new URL(base)
assert.equal(url.hostname, '127.0.0.1')
assert.equal(url.protocol, 'http:')
const browser = await chromium.launch({ headless: true })
const errors = []
try {
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    serviceWorkers: 'block',
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `channel${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  const snapshot = await (await context.request.get(`${base}/api/v1/bootstrap`)).json()
  const network = snapshot.networks[0]
  const account = (await signup.json()).account
  const original = snapshot.conversations.find(
    (item) => item.networkId === network.id && item.kind === 'channel',
  )
  const party = {
    id: 'legacy-fixture',
    conversationId: original.id,
    networkId: network.id,
    name: 'Legacy friends',
    description: 'Existing group',
    ownerId: 'legacy-owner',
    state: 'active',
    revision: 1,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    members: [
      {
        id: 'legacy-owner',
        backendAccountId: account.id,
        accountId: account.id,
        username: account.username,
        servicesAccount: network.account ?? account.username,
        displayName: 'Owner',
        joinedAt: '2026-01-01T00:00:00Z',
      },
    ],
  }
  snapshot.capabilities.parties = {
    networkId: network.id,
    maxMembers: 32,
    maxNameLength: 100,
    maxDescriptionLength: 500,
  }
  original.party = party
  await context.route('**/api/v1/bootstrap', (route) => route.fulfill({ json: snapshot }))
  let socket
  await context.routeWebSocket('**/api/v1/events**', (connection) => {
    socket = connection
    socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: snapshot.cursor }))
  })
  let conversionAttempts = 0
  await context.route('**/api/v1/parties/legacy-fixture/convert', async (route) => {
    conversionAttempts++
    if (conversionAttempts === 1)
      return route.fulfill({
        json: {
          status: 'pending',
          conversationId: original.id,
          prerequisite: 'Accept ownership in services.',
          warnings: [],
        },
      })
    const send = (type, data) =>
      socket.send(JSON.stringify({ v: 1, seq: ++snapshot.cursor, type, data }))
    send('party.updated', { ...party, state: 'converted', revision: 2 })
    const channel = { ...original }
    delete channel.party
    send('conversation.updated', channel)
    send('party.updated', party)
    await route.fulfill({
      json: { status: 'converted', conversationId: original.id, warnings: [] },
    })
  })
  const requests = []
  let partial = false
  let refuse = true
  await context.route('**/api/v1/networks/*/channels/availability**', async (route) => {
    const name = new URL(route.request().url()).searchParams.get('name')
    if (name === '#slow') await new Promise((resolve) => setTimeout(resolve, 900))
    await route
      .fulfill({
        json: {
          status:
            name === '#taken' ? 'unavailable' : name === '#available' ? 'available' : 'unknown',
        },
      })
      .catch(() => {})
  })
  await context.route('**/api/v1/networks/*/channels', async (route) => {
    if (route.request().method() !== 'POST') return route.continue()
    requests.push({
      path: new URL(route.request().url()).pathname,
      body: route.request().postDataJSON(),
    })
    if (refuse) {
      refuse = false
      return route.fulfill({ status: 409, json: { detail: 'The name is no longer available.' } })
    }
    if (partial) await new Promise((resolve) => setTimeout(resolve, 500))
    const response = await route.fetch()
    const body = await response.json()
    if (partial)
      Object.assign(body, {
        registered: false,
        configured: false,
        warnings: ['Review registration.'],
      })
    await route.fulfill({ response, json: body })
  })
  const page = await context.newPage()
  page.setDefaultTimeout(8000)
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(base)
  const sidebar = page.locator('.conversation-sidebar')
  const menu = sidebar.getByRole('button', { name: 'Channel actions' })
  await menu.waitFor()
  await page.locator('.network-tab').filter({ hasText: network.name }).click()
  assert.equal(await sidebar.getByRole('button', { name: 'New group conversation' }).count(), 0)
  await sidebar.getByRole('button', { name: /Jules/ }).click()
  assert.equal(await page.getByRole('button', { name: 'Add people' }).count(), 0)
  await menu.hover()
  const joinItem = page.getByRole('menuitem', { name: 'Join existing channel' })
  await joinItem.waitFor()
  await page.waitForTimeout(1200)
  assert.equal(await joinItem.isVisible(), true, 'hover menu stays open over the trigger')
  await joinItem.hover()
  await page.waitForTimeout(700)
  assert.equal(await joinItem.isVisible(), true, 'hover menu stays open over its content')
  await page.mouse.move(1000, 800)
  await joinItem.waitFor({ state: 'hidden' })
  await menu.focus()
  await page.keyboard.press('Enter')
  await page.keyboard.press('Escape')
  await menu.focus()
  await page.keyboard.press('Enter')
  await page.getByRole('menuitem', { name: 'Join existing channel' }).click()
  let dialog = page.getByRole('dialog', { name: 'Join a channel' })
  await dialog.getByLabel('Channel name').fill('notlisted')
  assert.equal(await dialog.getByText(/Create #/).count(), 0)
  await dialog.getByRole('button', { name: 'Close dialog' }).click()
  async function openCreate() {
    await menu.click()
    await page.getByRole('menuitem', { name: 'Create new channel' }).click()
    return page.getByRole('dialog', { name: 'Create new channel' })
  }
  dialog = await openCreate()
  await mkdir('../.data/channel-review', { recursive: true })
  await page.screenshot({ path: '../.data/channel-review/desktop.png' })
  assert.equal(await dialog.getByLabel('Invite only (+i)').isChecked(), true)
  await dialog.getByLabel('Announcements', { exact: true }).check()
  assert.equal(await dialog.getByLabel('Moderated (+m)').isChecked(), true)
  await dialog.getByLabel('Moderated (+m)').uncheck()
  await dialog.getByLabel('Channel name').fill('available')
  await dialog.getByText(/appears available/).waitFor()
  await dialog.getByLabel('Channel name').fill('slow')
  await page.waitForTimeout(400)
  await dialog.getByLabel('Channel name').fill('taken')
  await dialog.getByText(/name is unavailable/).waitFor()
  await page.waitForTimeout(700)
  assert.equal(
    await dialog.getByText(/name is unavailable/).isVisible(),
    true,
    'a stale response cannot replace the latest result',
  )
  assert.equal(await dialog.getByRole('button', { name: 'Create channel' }).isDisabled(), true)
  await dialog.getByLabel('Channel name').fill('fixturechannel')
  await dialog.getByText(/could not be confirmed/).waitFor()
  await dialog.getByLabel('Topic (optional)').fill('Fixture topic')
  await dialog.getByLabel('Channel key (optional)').fill('fixture-secret')
  await mkdir('../.data/channel-review', { recursive: true })
  await page.screenshot({ path: '../.data/channel-review/desktop.png' })
  await dialog.getByRole('button', { name: 'Create channel' }).click()
  await dialog.getByText('The name is no longer available.').waitFor()
  assert.equal(await dialog.getByLabel('Channel name').inputValue(), 'fixturechannel')
  assert.equal(await dialog.getByLabel('Channel key (optional)').inputValue(), 'fixture-secret')
  // A network change from elsewhere must not retarget the open form.
  await page
    .locator('.network-tab')
    .nth(1)
    .evaluate((button) => button.click())
  await dialog.getByRole('button', { name: 'Create channel' }).click()
  await dialog.waitFor({ state: 'hidden' })
  assert.equal(requests[1].path, `/api/v1/networks/${network.id}/channels`)
  assert.equal(requests[1].body.name, '#fixturechannel')
  assert.equal(requests[1].body.flags.split('-')[0].includes('m'), false)
  assert.equal(requests[1].body.flags.split('-')[1].includes('m'), true)
  assert.equal(requests[1].body.topic, 'Fixture topic')
  assert.equal((await page.getByText('fixturechannel', { exact: true }).count()) > 0, true)
  dialog = await openCreate()
  partial = true
  await dialog.getByLabel('Channel name').fill('partialchannel')
  await dialog.getByText(/could not be confirmed/).waitFor()
  await dialog.getByRole('button', { name: 'Create channel' }).click()
  assert.equal(await dialog.getByRole('button', { name: 'Creating…' }).isDisabled(), true)
  await dialog.getByText(/channel was created/).waitFor()
  await dialog.getByText('Review registration.').waitFor()
  assert.equal(await dialog.getByRole('button', { name: 'Create channel' }).count(), 0)
  await dialog.getByRole('button', { name: 'Open channel' }).click()
  await dialog.waitFor({ state: 'hidden' })
  assert.equal((await page.getByText('partialchannel', { exact: true }).count()) > 0, true)
  await page.locator('.network-tab').nth(2).click()
  dialog = await openCreate()
  await dialog.getByText(/cannot confirm channel creation/).waitFor()
  assert.equal(await dialog.getByRole('button', { name: 'Create channel' }).isDisabled(), true)
  await dialog.getByRole('button', { name: 'Close dialog' }).click()
  await page.locator('.network-tab').filter({ hasText: network.name }).click()
  await sidebar.getByText('Legacy friends', { exact: true }).click()
  await page
    .locator('.timeline-scroll')
    .waitFor()
    .catch(async (error) => {
      console.log(
        'Timeline unavailable:',
        page.url(),
        errors,
        (await page.locator('body').innerText()).slice(0, 1800),
      )
      throw error
    })
  await page.getByText('Make yourself at home. Send a message and I’ll echo it back. 👋').waitFor()
  await page.locator('.chat-header').getByRole('button', { name: 'Legacy friends' }).click()
  const details = page.getByRole('dialog', { name: 'Legacy friends' })
  await details.getByRole('button', { name: 'Convert to channel' }).click()
  await details.getByText(/Accept ownership in services/).waitFor()
  await details.getByRole('button', { name: 'Convert to channel' }).click()
  await details.waitFor({ state: 'hidden' })
  await page
    .locator('.timeline-scroll')
    .waitFor()
    .catch(async (error) => {
      console.log(
        'Converted timeline unavailable:',
        page.url(),
        errors,
        (await page.locator('body').innerText()).slice(0, 1800),
      )
      throw error
    })
  await sidebar.getByText(original.name, { exact: true }).waitFor()
  await page.getByText('Make yourself at home. Send a message and I’ll echo it back. 👋').waitFor()
  assert.equal(await sidebar.getByText('Legacy friends', { exact: true }).count(), 0)
  assert.equal((await sidebar.getByText(original.name, { exact: true }).count()) > 0, true)
  assert.deepEqual(errors, [])
  await context.close()

  const mobile = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
    serviceWorkers: 'block',
  })
  const signupMobile = await mobile.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `mobile${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signupMobile.status(), 201)
  const phone = await mobile.newPage()
  phone.setDefaultTimeout(8000)
  await phone.goto(base)
  await phone
    .getByRole('button', { name: /open conversations/i })
    .first()
    .click()
  await phone.getByRole('button', { name: 'Channel actions' }).tap()
  await phone.getByRole('menuitem', { name: 'Create new channel' }).tap()
  await phone.getByRole('dialog', { name: 'Create new channel' }).waitFor()
  await phone.screenshot({ path: '../.data/channel-review/mobile.png' })
  await mobile.close()
  console.log('Channel creation browser checks passed.')
} finally {
  await browser.close()
}
