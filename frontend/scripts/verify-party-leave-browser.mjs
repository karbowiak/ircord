// Isolated browser fixture: API and socket traffic never reaches the real backend.
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { createFixtures } from '../server/utils/fixtures.ts'
import { defaultNotificationSettings } from '../shared/notifications.ts'

const base = 'http://127.0.0.1:3000'
const snapshot = createFixtures().bootstrap
snapshot.demo = true
const network = snapshot.networks[0]
snapshot.capabilities.parties = { networkId: network.id, maxMembers: 32, maxNameLength: 100, maxDescriptionLength: 500 }
const member = { id: 'irc:alice', backendAccountId: 'alice', servicesAccount: 'alice', servicesRegisteredAt: '2026-01-01T00:00:00Z', username: 'alice', displayName: 'Alice', joinedAt: '2026-01-01T00:00:00Z' }
const party = {
  id: 'leave-target', conversationId: 'leave-conversation', networkId: network.id,
  name: 'Leave target', description: '', ownerId: member.id, members: [member],
  state: 'active', revision: 1, createdAt: member.joinedAt, updatedAt: member.joinedAt,
}
const conversation = {
  ...snapshot.conversations[0], id: party.conversationId, name: 'private-target',
  target: '#private-target', party, unreadCount: 0, mentionCount: 0,
  memberIds: ['you'], memberModes: {}, joined: true,
}
snapshot.conversations.push(conversation)
const activeConversation = snapshot.conversations.find((c) => c.networkId === network.id && c.kind === 'channel' && !c.party)
const requests = []
const errors = []
let failLeave = true
let socket
const browser = await chromium.launch({ headless: true })
let page
try {
  const context = await browser.newContext({ serviceWorkers: 'block', viewport: { width: 1280, height: 900 } })
  await context.route('**/*', (route) => new URL(route.request().url()).origin === base ? route.continue() : route.abort())
  await context.routeWebSocket('**/api/v1/events**', (connection) => {
    socket = connection
    connection.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: snapshot.cursor }))
  })
  await context.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (!path.startsWith('/api/')) return route.continue()
    let body = {}
    if (path.endsWith('/auth/session')) body = { account: { id: 'alice', username: 'alice' } }
    else if (path.endsWith('/auth/registration')) body = { enabled: true }
    else if (path.endsWith('/bootstrap')) body = snapshot
    else if (path.endsWith('/notifications/settings')) body = defaultNotificationSettings
    else if (path.endsWith('/parties/leave-target/leave')) {
      requests.push({ method: request.method(), path })
      if (failLeave) return route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ detail: 'The party is reconnecting. Try again shortly.' }) })
      snapshot.conversations = snapshot.conversations.filter((c) => c.id !== conversation.id)
      socket.send(JSON.stringify({ v: 1, seq: ++snapshot.cursor, type: 'party.removed', data: { id: party.id, conversationId: conversation.id, revision: ++party.revision } }))
    } else if (path.includes('/messages')) body = { items: [], nextCursor: null }
    else if (path.endsWith('/inbox/unread')) body = { unread: 0 }
    else if (path.endsWith('/saved/ids')) body = { items: [] }
    else if (path.endsWith('/app-data/appearance')) body = { settings: {} }
    else if (path.endsWith('/app-data/preferences')) body = { preferences: {} }
    else if (path.endsWith('/emotes')) body = { items: [], version: 0 }
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) })
  })
  page = await context.newPage()
  page.setDefaultTimeout(10000)
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(`${base}/channel/${activeConversation.id}`)
  await page.getByRole('heading', { name: activeConversation.name, exact: true, level: 1 }).waitFor()
  const target = page.locator('.conversation-sidebar').getByRole('button', { name: 'Leave target', exact: true })
  const openLeave = async () => {
    await target.click({ button: 'right' })
    const menu = page.getByRole('menu', { name: 'Leave target', exact: true })
    await menu.getByRole('menuitem', { name: 'Leave conversation', exact: true }).click()
    return page.getByRole('dialog', { name: 'Leave this group conversation?' })
  }
  // Cancelling must not mutate membership, or switch to the right-clicked group.
  await (await openLeave()).getByRole('button', { name: 'Cancel', exact: true }).click()
  assert.deepEqual(requests, [])
  assert.equal(page.url(), `${base}/channel/${activeConversation.id}`)
  assert(await target.isVisible())
  // Surface failures and retain the group so the person can retry.
  await (await openLeave()).getByRole('button', { name: 'Leave conversation', exact: true }).click()
  await page.getByRole('alert').filter({ hasText: 'The party is reconnecting. Try again shortly.' }).waitFor()
  assert(await target.isVisible())
  assert.deepEqual(requests, [{ method: 'POST', path: '/api/v1/parties/leave-target/leave' }])
  // Success removes the selected group after its membership event, not the active channel.
  failLeave = false
  await (await openLeave()).getByRole('button', { name: 'Leave conversation', exact: true }).click()
  await target.waitFor({ state: 'hidden' })
  assert.equal(requests.length, 2)
  assert.equal(page.url(), `${base}/channel/${activeConversation.id}`)
  assert.deepEqual(errors, [])
  console.log('Party leave menu: cancel, error, retry and non-active target passed; no real backend traffic.')
} catch (error) {
  console.error({ url: page?.url(), headings: await page?.getByRole('heading').allTextContents(), errors })
  throw error
} finally {
  await browser.close()
}
