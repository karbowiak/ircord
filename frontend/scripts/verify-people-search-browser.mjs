// All HTTP and socket traffic is intercepted; this uses the existing loopback frontend only.
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { createFixtures } from '../server/utils/fixtures.ts'
import { defaultNotificationSettings } from '../shared/notifications.ts'

const base = 'http://127.0.0.1:3000'
const snapshot = createFixtures().bootstrap
snapshot.demo = true
const one = {
  ...snapshot.members[1],
  id: 'libera:duplicate',
  nick: 'SameNick',
  displayName: 'Libera Person',
}
const two = {
  ...snapshot.members[1],
  id: 'ForeignId',
  nick: 'SameNick',
  displayName: 'Ergo Person',
}
snapshot.members.push(one, two)
const libera = snapshot.conversations.find((item) => item.networkId === 'libera')
const ergo = snapshot.conversations.find((item) => item.networkId === 'ergo')
libera.memberIds.push(one.id)
ergo.memberIds.push(two.id)
const created = []
const browser = await chromium.launch({ headless: true })
try {
  const context = await browser.newContext({
    serviceWorkers: 'block',
    viewport: { width: 1440, height: 900 },
  })
  await context.route('**/*', (route) =>
    new URL(route.request().url()).origin === base ? route.continue() : route.abort(),
  )
  await context.routeWebSocket('**/api/v1/events**', (socket) =>
    socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: snapshot.cursor })),
  )
  await context.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    if (!path.startsWith('/api/')) return route.continue()
    let body = {}
    if (path.endsWith('/auth/session')) body = { account: { id: 'fixture', username: 'fixture' } }
    else if (path.endsWith('/auth/registration')) body = { enabled: true }
    else if (path.endsWith('/bootstrap')) body = snapshot
    else if (path.endsWith('/notifications/settings')) body = defaultNotificationSettings
    else if (path.endsWith('/people'))
      body = { items: [], source: 'known', status: 'ready', more: false }
    else if (path.endsWith('/conversations') && request.method() === 'POST') {
      created.push(request.postDataJSON())
      body = { ...libera, id: 'fixture-direct', kind: 'direct', memberIds: ['you', one.id] }
    } else if (path.includes('/messages')) body = { items: [], nextCursor: null }
    else if (path.endsWith('/app-data/appearance')) body = { settings: {} }
    else if (path.endsWith('/app-data/preferences')) body = { preferences: {} }
    else if (path.endsWith('/app-data/library')) body = {}
    else if (path.endsWith('/emotes')) body = { items: [], version: 0 }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify(body),
    })
  })
  const page = await context.newPage()
  page.setDefaultTimeout(10000)
  const errors = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(`${base}/search`)
  const search = page.getByPlaceholder('Channels, people or messages — try from: or has:')
  await search.fill('SameNick')
  await page.getByText('Libera Person', { exact: true }).waitFor()
  assert.equal(await page.getByText('Ergo Person', { exact: true }).count(), 0)
  await page.getByText('Libera Person', { exact: true }).click()
  assert.deepEqual(created[0], { networkId: 'libera', nickname: 'SameNick' })
  await page.goto(`${base}/search`)
  await search.fill('ForeignId')
  await page.getByText('ForeignId', { exact: true }).click()
  assert.deepEqual(created[1], { networkId: 'libera', nickname: 'ForeignId' })
  assert.deepEqual(errors, [])
  console.log(
    'Search scopes duplicate nicknames and sends explicit nickname targets even when text matches a foreign member ID.',
  )
} finally {
  await browser.close()
}
