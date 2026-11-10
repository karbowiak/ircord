// All API and socket traffic is intercepted; no request reaches the user's backend.
import assert from 'node:assert/strict'
import { mkdir } from 'node:fs/promises'
import { chromium } from 'playwright'
import { createFixtures } from '../server/utils/fixtures.ts'
import { defaultNotificationSettings } from '../shared/notifications.ts'

const base = process.env.IRCORD_PARTY_BASE ?? 'http://127.0.0.1:3000'
const production = process.env.IRCORD_PARTY_PRODUCTION === '1'
const target = new URL(base)
if (
  target.protocol !== 'http:' ||
  target.hostname !== '127.0.0.1' ||
  target.pathname !== '/' ||
  target.search ||
  target.hash
)
  throw new Error('The party fixture requires a loopback HTTP origin')
const capture = production ? 'production-race-' : 'race-'
const skipCreateRace = process.env.IRCORD_PARTY_SKIP_CREATE_RACE === '1'
const pickerOnly = process.env.IRCORD_PARTY_PICKER_ONLY === '1'
const fixture = createFixtures()
const snapshot = fixture.bootstrap
const network = snapshot.networks[0]
snapshot.demo = true
snapshot.capabilities.parties = {
  networkId: network.id,
  maxMembers: 32,
  maxNameLength: 100,
  maxDescriptionLength: 500,
}
const account = { id: 'alice', username: 'alice' }
const candidates = [
  { accountId: 'bob', username: 'bob', displayName: 'Bob' },
  { accountId: 'carol', username: 'carol', displayName: 'Carol' },
  { accountId: 'jules', username: 'jules', displayName: 'Jules' },
]
if (pickerOnly) {
  candidates[1].displayName = 'Caz Parker'
  candidates.push({ accountId: 'alice', username: 'alice', displayName: 'Alice' })
  for (let i = 0; i < 12; i++)
    candidates.push({ accountId: `person${i}`, username: `person${i}`, displayName: `Person ${i}` })
}
for (const candidate of candidates)
  Object.assign(candidate, {
    id: `principal-${candidate.accountId}`,
    servicesAccount: candidate.username,
    servicesRegisteredAt: '2026-01-01T00:00:00Z',
    backendAccountId: candidate.accountId,
  })
const candidateSearches = []
const party = {
  id: 'fixture-party',
  conversationId: 'fixture-conversation',
  networkId: network.id,
  name: 'Weekend plans',
  description: 'Make plans together',
  ownerId: 'principal-alice',
  state: 'active',
  revision: 1,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  members: [
    {
      id: 'principal-alice',
      accountId: 'alice',
      backendAccountId: 'alice',
      servicesAccount: 'alice',
      servicesRegisteredAt: '2026-01-01T00:00:00Z',
      username: 'alice',
      displayName: 'Alice',
      joinedAt: '2026-01-01T00:00:00Z',
    },
  ],
}
const partyConversation = {
  ...snapshot.conversations[0],
  id: party.conversationId,
  name: 'secret-transport',
  target: '#secret-transport',
  networkId: network.id,
  party,
  unreadCount: 0,
  mentionCount: 0,
  memberIds: ['you'],
  memberModes: {},
  joined: true,
}
let mutations = []
let liveSocket
let releasePatch
let patchStarted
const startedPatch = new Promise((resolve) => {
  patchStarted = resolve
})
let releaseCreate
let createStarted
const startedCreate = new Promise((resolve) => {
  createStarted = resolve
})
let holdCreateBootstrap = false
let releaseAccept
let acceptStarted
const startedAccept = new Promise((resolve) => {
  acceptStarted = resolve
})
function waitFor(promise, description) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error(`${description} did not start`)), 8000)
    promise.then(() => {
      clearTimeout(timeout)
      resolve()
    })
  })
}
function event(type, data) {
  liveSocket.send(JSON.stringify({ v: 1, seq: ++snapshot.cursor, type, data }))
}
const browser = await chromium.launch({ headless: true })
const errors = []
try {
  const context = await browser.newContext({
    serviceWorkers: 'block',
    hasTouch: pickerOnly,
    viewport: { width: 1440, height: 900 },
  })
  await context.route('**/*', (route) => {
    const url = new URL(route.request().url())
    return url.origin === base ? route.continue() : route.abort()
  })
  await context.routeWebSocket('**/api/v1/events**', (socket) => {
    liveSocket = socket
    socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: snapshot.cursor }))
  })
  await context.route('**/api/**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    const method = request.method()
    if (!path.startsWith('/api/')) return route.continue()
    let body = {}
    if (path.endsWith('/auth/session')) body = { account }
    else if (path.endsWith('/auth/registration')) body = { enabled: true }
    else if (path.endsWith('/bootstrap')) {
      if (holdCreateBootstrap) {
        holdCreateBootstrap = false
        // Capture stale HTTP state before the newer socket event is delivered.
        body = {
          ...snapshot,
          conversations: snapshot.conversations.map((c) =>
            c.id === partyConversation.id ? { ...c, joined: false } : c,
          ),
        }
        event('conversation.updated', partyConversation)
        await new Promise((resolve) => {
          releaseCreate = resolve
          createStarted()
        })
      } else body = snapshot
    } else if (path.endsWith('/notifications/settings')) body = defaultNotificationSettings
    else if (path.endsWith('/people')) {
      const search = (url.searchParams.get('query') ?? '').toLowerCase()
      candidateSearches.push(search)
      if (pickerOnly && search === 'slow') await new Promise((resolve) => setTimeout(resolve, 700))
      if (pickerOnly && search === 'error')
        return route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({
            detail: 'Party account lookup is unavailable. Try again shortly.',
          }),
        })
      if (pickerOnly && search === 'mixed')
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            source: 'registered',
            status: 'partial',
            more: false,
            items: [
              {
                key: 'native',
                networkId: network.id,
                servicesAccount: 'native-account',
                displayName: 'Native',
                presence: 'unknown',
                eligible: true,
              },
              {
                key: 'unverified',
                networkId: network.id,
                nickname: 'Unknown',
                displayName: 'Unknown',
                presence: 'offline',
                eligible: false,
                reason: 'identity_unverified',
              },
            ],
          }),
        })
      body = {
        source: 'registered',
        status: 'ready',
        more: false,
        items: candidates
          .filter(
            (c) =>
              c.username.toLowerCase().includes(search) ||
              c.displayName.toLowerCase().includes(search),
          )
          .map((c) => ({
            key: `account:${c.username}`,
            networkId: network.id,
            servicesAccount: c.username,
            backendAccountId: c.username === 'carol' ? undefined : c.accountId,
            displayName: c.displayName,
            presence: 'unknown',
            eligible: true,
          })),
      }
    } else if (path.endsWith('/parties') && method === 'POST') {
      mutations.push({ path, body: request.postDataJSON() })
      snapshot.conversations.push(partyConversation)
      holdCreateBootstrap = !skipCreateRace
      body = party
    } else if (path.endsWith('/parties/fixture-party') && method === 'PATCH') {
      mutations.push({ path, body: request.postDataJSON() })
      Object.assign(party, request.postDataJSON(), { revision: party.revision + 1 })
      body = party
    } else if (path.endsWith('/parties/fixture-party/invitations') && method === 'POST') {
      mutations.push({ path, body: request.postDataJSON() })
      body = {
        id: 'invite-1',
        networkId: network.id,
        channel: '',
        inviterNick: 'Alice',
        party: {
          id: party.id,
          name: party.name,
          description: party.description,
          inviterAccountId: 'alice',
          memberCount: 1,
        },
      }
    } else if (path.endsWith('/parties/fixture-party/owner') && method === 'POST') {
      mutations.push({ path, body: request.postDataJSON() })
      Object.assign(party, { ownerId: 'principal-bob', revision: party.revision + 1 })
      body = party
    } else if (path.endsWith('/parties/fixture-party/members/principal-bob') && method === 'DELETE') {
      mutations.push({ path })
      Object.assign(party, {
        members: party.members.filter((m) => m.id !== 'principal-bob'),
        revision: party.revision + 1,
      })
      body = party
    } else if (path.endsWith('/parties/accepted-party') && method === 'PATCH') {
      mutations.push({ path })
      await new Promise((resolve) => {
        releasePatch = resolve
        patchStarted()
      })
      body = {
        ...party,
        id: 'accepted-party',
        conversationId: 'accepted-conversation',
        name: 'Delayed',
        revision: 4,
      }
    } else if (path.endsWith('/invitations/declined') && method === 'DELETE') {
      mutations.push({ path })
    } else if (path.endsWith('/invitations/received/accept') && method === 'POST') {
      mutations.push({ path })
      const live = {
        ...partyConversation,
        id: 'accepted-conversation',
        joined: true,
        party: {
          ...party,
          id: 'accepted-party',
          conversationId: 'accepted-conversation',
          name: 'Friends',
          revision: 4,
        },
      }
      body = { ...live, joined: false }
      snapshot.conversations.push(live)
      event('conversation.updated', live)
      await new Promise((resolve) => {
        releaseAccept = resolve
        acceptStarted()
      })
    } else if (path.includes('/messages')) body = { items: [], nextCursor: null }
    else if (path.endsWith('/inbox/unread')) body = { unread: 0 }
    else if (path.endsWith('/saved/ids')) body = { items: [] }
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
  page.on('pageerror', (error) => errors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error' && !message.text().startsWith('Failed to load resource'))
      errors.push(message.text().split('\n')[0])
  })
  page.setDefaultTimeout(8000)
  await page.goto(base)
  if (pickerOnly) {
    await page
      .locator('.conversation-sidebar')
      .getByRole('button', { name: 'New group conversation' })
      .click()
    const dialog = page.getByRole('dialog', { name: 'New group conversation' })
    const picker = dialog.getByRole('combobox', { name: 'Find people by name or username' })
    await picker.focus()
    await dialog.getByRole('option', { name: 'Bob bob', exact: true }).waitFor()
    const list = dialog.getByRole('listbox')
    const rect = await list.boundingBox()
    const touch = await context.newCDPSession(page)
    const x = rect.x + rect.width / 2
    const startY = rect.y + rect.height - 25
    await touch.send('Input.dispatchTouchEvent', {
      type: 'touchStart',
      touchPoints: [{ x, y: startY }],
    })
    for (let offset = 20; offset <= 100; offset += 20) {
      await touch.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x, y: startY - offset }],
      })
      await page.waitForTimeout(20)
    }
    await touch.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] })
    await page.waitForFunction(
      (id) => document.getElementById(id)?.scrollTop > 0,
      await list.getAttribute('id'),
    )
    await list.evaluate((element) => {
      element.scrollTop = 0
    })
    await touch.detach()
    assert.equal(await dialog.getByRole('option', { name: 'Alice alice', exact: true }).count(), 0)
    assert.equal(await picker.getAttribute('aria-expanded'), 'true')
    assert.equal(await picker.getAttribute('aria-activedescendant'), null)
    await picker.press('ArrowDown')
    assert.equal(
      await dialog
        .getByRole('option', { name: 'Bob bob', exact: true })
        .getAttribute('aria-selected'),
      'true',
    )
    await picker.press('ArrowUp')
    const last = dialog.getByRole('option', { name: 'Person 11 person11', exact: true })
    assert.equal(await last.getAttribute('aria-selected'), 'true')
    await page.waitForFunction(
      (id) => {
        const el = document.getElementById(id)
        return (
          el &&
          el.getBoundingClientRect().bottom <= el.parentElement.getBoundingClientRect().bottom + 1
        )
      },
      await last.getAttribute('id'),
    )
    await picker.press('Escape')
    assert(await dialog.isVisible())
    assert.equal(await picker.getAttribute('aria-expanded'), 'false')
    assert.equal(await picker.getAttribute('aria-controls'), null)
    await page.keyboard.type('bo')
    await dialog.getByRole('option', { name: 'Bob bob', exact: true }).waitFor()
    await picker.fill('')
    await picker.press('Escape')
    await picker.click()
    await dialog.getByRole('option', { name: 'Bob bob', exact: true }).waitFor()
    await picker.press('ArrowUp')
    assert.equal(await last.getAttribute('aria-selected'), 'true')
    await picker.fill('par')
    assert.equal(await dialog.getByRole('option', { name: 'Bob bob', exact: true }).count(), 0)
    await dialog.getByRole('option', { name: 'Caz Parker carol', exact: true }).waitFor()
    await picker.press('ArrowUp')
    await picker.press('Enter')
    assert(
      await dialog.getByRole('button', { name: 'Remove Caz Parker from selection' }).isVisible(),
    )
    assert(await dialog.isVisible())
    assert.equal(await picker.getAttribute('aria-expanded'), 'false')
    const countAfterSelection = candidateSearches.length
    await page.waitForTimeout(300)
    assert.equal(
      candidateSearches.length,
      countAfterSelection,
      'selection must not start an empty search',
    )
    await page.keyboard.type('bo')
    await dialog.getByRole('option', { name: 'Bob bob', exact: true }).waitFor()
    await picker.fill('')
    await picker.press('Escape')
    await picker.click()
    await dialog.getByRole('option', { name: 'Bob bob', exact: true }).waitFor()
    assert.equal(
      await dialog.getByRole('option', { name: 'Caz Parker carol', exact: true }).count(),
      0,
    )
    await picker.fill('slow')
    await page.waitForTimeout(250)
    await picker.fill('zzz')
    await dialog.getByText('No matching accounts.').waitFor()
    await page.waitForTimeout(600)
    assert(await dialog.getByText('No matching accounts.').isVisible())
    await picker.fill('error')
    await dialog.getByText('Party account lookup is unavailable. Try again shortly.').waitFor()
    await picker.fill('mixed')
    await dialog.getByText('Some accounts may be missing. Try again.').waitFor()
    assert.equal(
      await dialog
        .getByRole('option', { name: 'Unknown Account could not be verified.' })
        .getAttribute('aria-disabled'),
      'true',
    )
    await picker.press('ArrowDown')
    await picker.press('Enter')
    assert(await dialog.getByRole('button', { name: 'Remove Native from selection' }).isVisible())
    await picker.fill('bo')
    await dialog.getByRole('option', { name: 'Bob bob', exact: true }).tap()
    assert(await dialog.getByRole('button', { name: 'Remove Bob from selection' }).isVisible())
    assert(await picker.evaluate((el) => document.activeElement === el))
    await page.keyboard.type('jul')
    await dialog.getByRole('option', { name: 'Jules jules', exact: true }).waitFor()
    await picker.press('Tab')
    assert.equal(await picker.getAttribute('aria-expanded'), 'false')
    assert.equal(await dialog.getByRole('listbox').count(), 0)
    await picker.focus()
    await dialog.getByRole('option', { name: 'Jules jules', exact: true }).waitFor()
    await dialog.getByRole('textbox', { name: 'Name (optional)' }).focus()
    assert.equal(await picker.getAttribute('aria-expanded'), 'false')
    await picker.focus()
    await picker.fill('cancelled')
    await picker.press('Escape')
    await page.waitForTimeout(300)
    assert(!candidateSearches.includes('cancelled'), 'closing must cancel the debounce')
    await picker.press('Escape')
    await dialog.waitFor({ state: 'hidden' })
    assert.deepEqual(mutations, [])
    assert.deepEqual(errors, [])
    console.log(
      'Party picker focus, filtering, exclusion, keyboard, scrolling, errors, stale requests, pointer selection and dismissal passed.',
    )
    await context.close()
    process.exitCode = 0
  } else {
    // The source worker URL only exists on Vite; its IndexedDB behavior is covered in dev mode.
    if (!production)
      await page.evaluate(
        async ({ snapshot, conversation, party }) => {
          const worker = new Worker('/_nuxt/workers/offline.worker.ts?worker_file&type=module', {
            type: 'module',
          })
          const pending = new Map()
          let serial = 0
          worker.onmessage = ({ data }) => {
            const item = pending.get(data.id)
            pending.delete(data.id)
            if (data.error) item.reject(new Error(data.error))
            else item.resolve(data.result)
          }
          const rpc = (method, ...args) =>
            new Promise((resolve, reject) => {
              const id = ++serial
              pending.set(id, { resolve, reject })
              worker.postMessage({ id, method, args })
            })
          try {
            await rpc('open', `party-fixture-${crypto.randomUUID()}`)
            const state = { ...snapshot, cursor: 0, conversations: [conversation] }
            await rpc('begin', state, new Date().toISOString(), 'fixture')
            await rpc(
              'page',
              [
                {
                  id: 'private-message',
                  conversationId: conversation.id,
                  senderId: 'bob',
                  kind: 'message',
                  text: 'Private fixture',
                  createdAt: new Date().toISOString(),
                  attachment: {
                    url: 'https://example.test/a.png',
                    name: 'a.png',
                    contentType: 'image/png',
                  },
                },
              ],
              0,
              'fixture',
            )
            await rpc('saveFile', 'https://example.test/a.png', new Blob(['fixture']))
            await rpc('events', [
              {
                v: 1,
                seq: 1,
                type: 'party.removed',
                data: { id: party.id, conversationId: conversation.id, revision: 2 },
              },
            ])
            await rpc('events', [
              { v: 1, seq: 2, type: 'conversation.updated', data: conversation },
            ])
            const status = await rpc('status')
            if (
              status.messages !== 0 ||
              status.files !== 0 ||
              status.meta.state.conversations.length !== 0
            )
              throw new Error('Removed party returned to the offline archive')
          } finally {
            await rpc('clear')
            worker.terminate()
          }
        },
        { snapshot, conversation: partyConversation, party },
      )
    const sidebar = page.locator('.conversation-sidebar')
    await sidebar.getByRole('button', { name: 'New group conversation' }).click()
    await page.getByRole('dialog', { name: 'New group conversation' }).waitFor()
    await page.keyboard.press('Escape')
    await sidebar.getByRole('button', { name: /Jules/ }).click()
    await page.getByRole('button', { name: 'Add people' }).click()
    const fromDirect = page.getByRole('dialog', { name: 'New group conversation' })
    await fromDirect.getByRole('button', { name: 'Remove Jules from selection' }).waitFor()
    assert(
      await fromDirect.getByRole('button', { name: 'Remove Jules from selection' }).isDisabled(),
    )
    assert(await fromDirect.getByRole('button', { name: 'Create conversation' }).isDisabled())
    await page.keyboard.press('Escape')
    await page.waitForFunction(
      () => document.activeElement?.getAttribute('aria-label') === 'Add people',
    )
    await page.getByRole('button', { name: 'Add people' }).click()
    await fromDirect.getByRole('button', { name: 'Remove Jules from selection' }).waitFor()
    const dialog = fromDirect
    const picker = dialog.getByRole('combobox', { name: 'Find people by name or username' })
    await picker.fill('bob')
    await dialog.getByRole('option', { name: /Bob/ }).waitFor()
    await picker.press('ArrowDown')
    await picker.press('Enter')
    assert(await dialog.getByRole('button', { name: 'Remove Bob from selection' }).isVisible())
    await dialog.getByRole('textbox', { name: 'Name (optional)' }).fill('Weekend plans')
    await dialog.getByRole('button', { name: 'Create conversation' }).click()
    if (!skipCreateRace) {
      await waitFor(startedCreate, 'Delayed creation bootstrap')
      await sidebar.getByText('Weekend plans', { exact: true }).waitFor()
      releaseCreate()
    }
    await page.waitForURL(`${base}/dm/fixture-conversation`)
    await page.locator('.composer textarea').waitFor()
    assert(
      await page.locator('.composer textarea').isEnabled(),
      'A stale creation bootstrap must not undo the live JOIN',
    )
    assert.deepEqual(mutations[0].body.servicesAccounts, ['jules', 'bob'])
    assert(
      await sidebar.getByRole('button', { name: /Jules/ }).isVisible(),
      'the original DM remains available',
    )
    await page.reload()
    await page.waitForURL(`${base}/dm/fixture-conversation`)
    await page.getByRole('heading', { name: 'Weekend plans', level: 1 }).waitFor()
    assert.equal(await page.getByText('secret-transport', { exact: false }).count(), 0)
    await page.getByRole('button', { name: 'Conversation options' }).click()
    assert.equal(await page.getByRole('menuitem', { name: 'Channel settings' }).count(), 0)
    await page.keyboard.press('Escape')
    const before = await page.locator('.conversation-history').boundingBox()
    await page.locator('.chat-header').getByRole('button', { name: 'Weekend plans' }).click()
    const details = page.getByRole('dialog', { name: 'Weekend plans' })
    await mkdir('../.data/party-review', { recursive: true })
    await page.screenshot({ path: `../.data/party-review/${capture}desktop.png` })
    await page.emulateMedia({ colorScheme: 'dark' })
    await page.waitForFunction(() => document.documentElement.dataset.theme === 'dark')
    await page.screenshot({ path: `../.data/party-review/${capture}desktop-dark.png` })
    await page.emulateMedia({ colorScheme: 'light' })
    await details.getByRole('textbox', { name: 'Description (optional)' }).fill('Updated plans')
    await details.getByRole('button', { name: 'Save changes' }).click()
    await page.waitForFunction(() => document.querySelector('input') !== null)
    await details.getByRole('combobox', { name: 'Find people by name or username' }).fill('carol')
    await details.getByRole('option', { name: /Carol/ }).click()
    await details.getByRole('button', { name: 'Invite', exact: true }).click()
    await page.waitForFunction(
      () => !document.querySelector('[aria-label="Remove Carol from selection"]'),
    )
    assert(
      mutations.some(
        (entry) => entry.path.endsWith('/invitations') && entry.body.servicesAccount === 'carol',
      ),
    )
    party.members.push({ ...candidates[0], joinedAt: '2026-01-02T00:00:00Z' })
    party.revision++
    event('party.updated', { ...party })
    await details.getByRole('button', { name: 'Transfer ownership to Bob' }).click()
    await page
      .getByRole('dialog', { name: 'Transfer ownership to Bob?' })
      .getByRole('button', { name: 'Transfer ownership' })
      .click()
    assert(mutations.some((entry) => entry.path.endsWith('/owner') && entry.body.memberId === 'principal-bob' && !('accountId' in entry.body)))
    await details.getByText('bob · Owner').waitFor()
    assert.equal(
      await details.getByRole('button', { name: 'Remove Bob' }).count(),
      0,
      'non-owner cannot remove members',
    )
    party.ownerId = 'principal-alice'
    party.revision++
    event('party.updated', { ...party })
    await details.getByRole('button', { name: 'Remove Bob' }).click()
    await page
      .getByRole('dialog', { name: 'Remove Bob from this conversation?' })
      .getByRole('button', { name: 'Remove', exact: true })
      .click()
    await details.getByText('Bob', { exact: true }).waitFor({ state: 'hidden' })
    assert(mutations.some((entry) => entry.path.endsWith('/members/principal-bob')))
    party.state = 'recovering'
    party.revision++
    event('party.updated', { ...party })
    await details.getByText('Reconnecting this conversation…').waitFor()
    assert(await page.locator('.composer textarea').isDisabled())
    party.state = 'archived'
    party.revision++
    event('party.updated', { ...party })
    await details.getByText('This conversation is archived.').waitFor()
    assert(await details.getByRole('button', { name: 'Leave conversation' }).isDisabled())
    party.state = 'active'
    party.revision++
    event('party.updated', { ...party })
    await page.keyboard.press('Escape')
    await page.waitForFunction(() =>
      document.activeElement?.classList.contains('channel-name-button'),
    )
    const after = await page.locator('.conversation-history').boundingBox()
    assert.equal(before.height, after.height, 'dialog does not move the chat timeline')
    await page.setViewportSize({ width: 390, height: 844 })
    await page.locator('.chat-header').getByRole('button', { name: 'Weekend plans' }).click()
    await page.getByRole('dialog', { name: 'Weekend plans' }).waitFor()
    await page.screenshot({ path: `../.data/party-review/${capture}mobile.png` })
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.keyboard.press('Escape')
    await page.setViewportSize({ width: 1440, height: 900 })
    event('invitation.received', {
      id: 'declined',
      networkId: network.id,
      channel: '',
      inviterNick: 'Bob',
      party: {
        id: 'declined-party',
        name: 'Another group',
        description: '',
        inviterAccountId: 'bob',
        memberCount: 2,
      },
    })
    await page.locator('.invitation-banner').getByRole('button', { name: 'Decline' }).click()
    assert(mutations.some((entry) => entry.path.endsWith('/invitations/declined')))
    event('invitation.received', {
      id: 'received',
      networkId: network.id,
      channel: '',
      inviterNick: 'Bob',
      party: {
        id: 'accepted-party',
        name: 'Friends',
        description: '',
        inviterAccountId: 'bob',
        memberCount: 2,
      },
    })
    const banner = page.locator('.invitation-banner')
    await banner.getByText('Friends').waitFor()
    assert.equal(await banner.getByRole('button', { name: 'Accept' }).isEnabled(), true)
    assert(
      !mutations.some((entry) => entry.path.endsWith('/invitations/received/accept')),
      'receiving does not accept',
    )
    await banner.getByRole('button', { name: 'Accept' }).click()
    await waitFor(startedAccept, 'Delayed acceptance')
    await sidebar.getByText('Friends', { exact: true }).waitFor()
    await sidebar.getByText('Friends', { exact: true }).click()
    await page.waitForURL(`${base}/dm/accepted-conversation`)
    assert(
      await page.locator('.composer textarea').isEnabled(),
      'The socket JOIN must be applied before acceptance resolves',
    )
    releaseAccept()
    await banner.waitFor({ state: 'hidden' })
    await page.waitForURL(`${base}/dm/accepted-conversation`)
    assert(
      await page.locator('.composer textarea').isEnabled(),
      'A stale acceptance response must not undo the live JOIN',
    )
    await page.locator('.composer textarea').fill('Disposable draft')
    assert.equal(await page.locator('.composer textarea').inputValue(), 'Disposable draft')
    await page.locator('.chat-header').getByRole('button', { name: 'Friends' }).click()
    const acceptedDialog = page.getByRole('dialog', { name: 'Friends' })
    await acceptedDialog.getByRole('textbox', { name: 'Name (optional)' }).fill('Delayed')
    await acceptedDialog.getByRole('button', { name: 'Save changes' }).click()
    await page.waitForFunction(() => document.querySelector('[role="dialog"]') !== null)
    await waitFor(startedPatch, 'Delayed save')
    event('party.removed', {
      id: 'accepted-party',
      conversationId: 'accepted-conversation',
      revision: 5,
    })
    releasePatch()
    await page.waitForURL((url) => !url.pathname.includes('accepted-conversation'))
    assert.equal(await page.locator('.conversation-sidebar').getByText('Friends').count(), 0)
    assert.deepEqual(errors, [])
    console.log(
      `Mocked ${production ? 'production' : 'development'} creation and acceptance JOIN races, edit, invite, owner permissions, delayed removal${production ? '' : ' and IndexedDB removal'}, mobile dialog and timeline geometry passed.`,
    )
    await context.close()
  }
} finally {
  releaseCreate?.()
  releaseAccept?.()
  releasePatch?.()
  await browser.close()
}
