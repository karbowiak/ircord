// Browser checks for the channel guide (#258) against a production build with mocked API
// responses: the sidebar notice and Featured view in Join existing, Administration → Welcome
// with its request queue, and the channel operators' request tab, at desktop and phone widths.
// Run a non-demo build (`bun run build`, then `bun .output/server/index.mjs`) and point
// IRCORD_UI_URL at it.
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'

const url = process.env.IRCORD_UI_URL ?? 'http://127.0.0.1:3000'
const dataDir = resolve(process.cwd(), '..', '.data')
mkdirSync(dataDir, { recursive: true })

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

const conversation = (name, extra = {}) => ({
  id: `conv-${name}`,
  networkId: 'ircord',
  name,
  target: `#${name}`,
  kind: 'channel',
  joined: true,
  topic: '',
  unreadCount: 0,
  mentionCount: 0,
  favorite: false,
  memberIds: ['you'],
  ...extra,
})

function bootstrap() {
  const profile = { id: 'you', nick: 'veteran', displayName: 'Veteran', status: 'online' }
  return {
    cursor: 1,
    capabilities: { embeds: false },
    user: profile,
    networks: [
      {
        id: 'ircord',
        name: 'IRCord',
        initials: 'IR',
        color: 'mint',
        status: 'connected',
        host: 'irc.ircord.com',
        port: 6697,
        tls: true,
        nick: 'veteran',
        profile,
        account: 'veteran',
        auth: { method: 'sasl-plain', account: 'veteran', passwordSet: true },
        autoJoin: ['#general'],
        perform: [],
        capabilities: ['sasl'],
        isupport: { NETWORK: 'IRCord', PREFIX: '(qaohv)~&@%+', CASEMAPPING: 'rfc1459' },
        features: { history: true, readMarkers: true, messageLineBytes: 310 },
      },
    ],
    conversations: [
      conversation('general', { memberModes: { you: '@' }, topic: 'Say hi' }),
      conversation('eve'),
    ],
    members: [profile],
  }
}

const guideView = () => ({
  available: true,
  revision: 4,
  groups: [{ name: 'Games' }],
  entries: [
    { channel: '#general', state: 'everyone', description: 'Say hi', members: 40, nsfw: false },
    { channel: '#eve', group: 'Games', state: 'suggested', members: 12, nsfw: false },
    { channel: '#factorio', group: 'Games', state: 'suggested', members: 7, nsfw: false },
    { channel: '#food', state: 'listed', topic: 'What we cooked', members: 5, nsfw: false },
  ],
})

const adminGuide = () => ({
  guide: {
    groups: [{ name: 'Games' }],
    entries: [
      { channel: '#general', state: 'everyone', description: 'Say hi' },
      { channel: '#eve', group: 'Games', state: 'suggested' },
      { channel: '#gone', state: 'listed' },
    ],
  },
  revision: 4,
  saved: true,
  defaults: ['#general'],
  updatedAt: '2026-11-16T12:00:00Z',
  updatedBy: 'owner',
  directory: true,
  warnings: [{ channel: '#gone', kind: 'missing' }],
  requests: [
    {
      channel: '#art',
      requester: 'carol',
      group: 'Creative',
      description: 'Drawings and paintings',
      createdAt: '2026-11-16T10:00:00Z',
      members: 9,
    },
    { channel: '#x', requester: 'xena', createdAt: '2026-11-16T11:00:00Z', members: 2 },
  ],
})

const json = (route, body, status = 200) =>
  route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })

async function install(context, state) {
  await context.route('**/api/app/**', async (route) => {
    const req = route.request()
    const { pathname } = new URL(req.url())
    if (pathname === '/api/app/preferences' && req.method() === 'GET')
      return json(route, { preferences: state.preferences, updatedAt: null })
    if (pathname === '/api/app/preferences' && req.method() === 'PUT') {
      state.preferences = req.postDataJSON().preferences
      return json(route, { preferences: state.preferences, updatedAt: new Date().toISOString() })
    }
    return json(route, {})
  })
  await context.route('**/api/v1/**', async (route) => {
    const req = route.request()
    const { pathname } = new URL(req.url())
    const method = req.method()
    const record = (body) => state.calls.push({ method, pathname, body })
    if (pathname === '/api/v1/auth/session')
      return json(route, {
        account: {
          id: 'acc-veteran',
          username: 'veteran',
          createdAt: '2026-11-01T00:00:00Z',
          lastSeenAt: '2026-11-16T00:00:00Z',
          expiresAt: '2027-02-14T00:00:00Z',
        },
      })
    if (pathname === '/api/v1/bootstrap') return json(route, state.bootstrap)
    if (pathname.endsWith('/messages') && method === 'GET')
      return json(route, { items: [], nextCursor: null })
    if (pathname === '/api/v1/networks/ircord/channel-guide') return json(route, guideView())
    if (pathname === '/api/v1/networks/ircord/channel-guide/join') {
      const { channels } = req.postDataJSON()
      record(channels)
      if (state.failJoinOnce) {
        state.failJoinOnce = false
        return json(route, {
          joined: [],
          queued: [],
          failed: channels.map((channel) => ({ channel, reason: 'Banned' })),
        })
      }
      return json(route, {
        joined: channels.map((c) => conversation(c.slice(1))),
        queued: [],
        failed: [],
      })
    }
    if (pathname === '/api/v1/administration/access') return json(route, { owner: true })
    if (pathname === '/api/v1/administration/channel-guide' && method === 'GET')
      return json(route, state.admin)
    if (pathname === '/api/v1/administration/channel-guide' && method === 'PUT') {
      const body = req.postDataJSON()
      record(body)
      if (state.conflictOnce) {
        state.conflictOnce = false
        return json(route, { detail: 'Another owner changed the channel guide.' }, 409)
      }
      const listed = new Set(body.guide.entries.map((e) => e.channel.toLowerCase()))
      state.admin = {
        ...state.admin,
        guide: body.guide,
        revision: body.revision + 1,
        requests: state.admin.requests.filter((r) => !listed.has(r.channel.toLowerCase())),
      }
      return json(route, state.admin)
    }
    if (pathname === '/api/v1/administration/channel-guide/channels')
      return json(route, {
        items: [{ name: '#food', users: 5, topic: 'What we cooked', registered: true }],
      })
    const decline = /^\/api\/v1\/administration\/channel-guide\/requests\/(.+)$/.exec(pathname)
    if (decline && method === 'DELETE') {
      record(decodeURIComponent(decline[1]))
      state.admin.requests = state.admin.requests.filter(
        (r) => r.channel !== `#${decodeURIComponent(decline[1])}`,
      )
      return route.fulfill({ status: 204, body: '' })
    }
    if (pathname === '/api/v1/conversations/conv-general/channel-guide') {
      if (method === 'PUT') {
        const body = req.postDataJSON()
        record(body)
        state.listing = { ...state.listing, status: 'pending', ...body }
      }
      if (method === 'DELETE') {
        record(null)
        state.listing = { ...state.listing, status: 'none', group: '', description: '' }
      }
      return json(route, state.listing)
    }
    if (pathname.endsWith('/channel-guide'))
      return json(route, { available: false, operator: false, status: 'none', groups: [] })
    if (pathname === '/api/v1/inbox') return json(route, { items: [], unreadCount: 0 })
    if (pathname === '/api/v1/emotes') return json(route, { version: 1, emotes: [] })
    return json(route, {})
  })
}

function createState() {
  return {
    bootstrap: bootstrap(),
    admin: adminGuide(),
    listing: { available: true, operator: true, status: 'none', groups: ['Games'] },
    preferences: { channelGuideSeen: false },
    calls: [],
  }
}

const noSideScroll = (page) =>
  page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)

const browser = await chromium.launch()
const results = {}
try {
  for (const [label, viewport] of [
    ['desktop', { width: 1280, height: 860 }],
    ['phone', { width: 390, height: 844 }],
  ]) {
    console.log(`Checking the notice and Featured (${label})`)
    const context = await browser.newContext({ viewport, serviceWorkers: 'block' })
    const state = createState()
    await install(context, state)
    const page = await context.newPage()
    page.on('pageerror', (error) => console.error('Browser error:', error.message))
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    if (label === 'phone')
      await page
        .getByRole('button', { name: /conversations/i })
        .first()
        .click()
    const notice = page.locator('.channel-guide-notice')
    await notice.waitFor({ state: 'visible' })
    await notice.getByRole('button', { name: 'Browse channels' }).click()
    const dialog = page.getByRole('dialog', { name: 'Join a channel' })
    await dialog.getByRole('tab', { name: 'Featured', selected: true }).waitFor()
    const row = (name) => dialog.locator(`.channel-guide-row[data-channel="${name}"]`)
    assert((await row('#eve').getByText('Joined').count()) === 1, '#eve shows as joined')
    assert(await row('#eve').getByRole('checkbox').isDisabled(), 'Joined channels are locked')
    assert(
      (await row('#factorio').getByRole('checkbox').getAttribute('aria-checked')) === 'false',
      'Featured ticks nothing for you',
    )
    assert(await noSideScroll(page), `Featured must not scroll sideways (${label})`)
    await row('#food').click()
    await page.screenshot({ path: resolve(dataDir, `channel-guide-featured-${label}.png`) })
    state.failJoinOnce = true
    await dialog.getByRole('button', { name: 'Join 1 channel' }).click()
    await dialog.locator('.form-error').waitFor({ state: 'visible' })
    assert(await dialog.isVisible(), 'A failed join keeps Join existing open')
    await dialog.getByRole('button', { name: 'Join 1 channel' }).click()
    await dialog.waitFor({ state: 'hidden' })
    const joins = state.calls.filter((c) => c.pathname.endsWith('/channel-guide/join'))
    assert(
      JSON.stringify(joins.map((c) => c.body)) === JSON.stringify([['#food'], ['#food']]),
      `Featured joins exactly the picked channel, got ${JSON.stringify(joins)}`,
    )
    await page.waitForTimeout(800)
    assert(state.preferences.channelGuideSeen === true, 'Opening the notice marks it seen')
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('.workspace').waitFor({ state: 'visible' })
    await page.waitForTimeout(1000)
    assert((await notice.count()) === 0, 'The notice shows only once')
    results[`featured-${label}`] = 'passed'
    await context.close()
  }

  console.log('Checking Administration → Welcome')
  {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 900 },
      serviceWorkers: 'block',
    })
    const state = createState()
    await install(context, state)
    const page = await context.newPage()
    page.on('pageerror', (error) => console.error('Browser error:', error.message))
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.getByRole('button', { name: 'Administration' }).click()
    const admin = page.getByRole('dialog', { name: 'Administration' })
    await admin.getByRole('button', { name: 'Welcome' }).click()
    const request = (name) => admin.locator(`.guide-requests li[data-channel="${name}"]`)
    await request('#art').waitFor()
    assert(
      (await request('#art').textContent()).includes('Drawings and paintings'),
      'Requests show the proposed description',
    )
    await admin.getByText('This channel doesn’t exist right now.', { exact: false }).waitFor()
    await page.screenshot({ path: resolve(dataDir, 'channel-guide-admin-queue.png') })

    // Decline the one-character #x, approve #art (creating its group), then add and save.
    await request('#x').getByRole('button', { name: 'Decline' }).click()
    await request('#x').waitFor({ state: 'detached' })
    assert(
      state.calls.some((c) => c.method === 'DELETE' && c.body === 'x'),
      'Declining a one-character channel calls the API',
    )
    await request('#art').getByRole('button', { name: 'Approve' }).click()
    await request('#art').waitFor({ state: 'detached' })
    const approved = state.calls.findLast((c) => c.method === 'PUT')?.body
    assert(
      approved?.revision === 4 &&
        approved.guide.groups.some((g) => g.name === 'Creative') &&
        approved.guide.entries.some(
          (e) =>
            e.channel === '#art' &&
            e.group === 'Creative' &&
            e.state === 'listed' &&
            e.description === 'Drawings and paintings',
        ),
      `Approving saves the request as a listed entry, got ${JSON.stringify(approved)}`,
    )
    await admin.getByLabel('Add a channel').fill('foo')
    await admin.locator('.guide-directory button', { hasText: '#food' }).click()
    await admin.getByLabel('How #food is offered').click()
    await page.getByRole('option', { name: 'Suggested' }).click()
    await admin.getByLabel('Description of #food').fill('What we cooked')
    const preview = admin.locator('.guide-preview')
    assert(
      (await preview
        .locator('.channel-guide-row[data-channel="#food"]')
        .getByRole('checkbox')
        .getAttribute('aria-checked')) === 'true',
      'The preview ticks suggested channels',
    )
    state.conflictOnce = true
    await admin.getByRole('button', { name: 'Save guide' }).click()
    await admin.getByText('Another owner changed the channel guide.', { exact: false }).waitFor()
    await admin.getByRole('button', { name: 'Save guide' }).click()
    await admin.getByText('Channel guide saved.').waitFor()
    const saved = state.calls.findLast((c) => c.method === 'PUT')?.body
    assert(
      saved.guide.entries.some(
        (e) =>
          e.channel === '#food' && e.state === 'suggested' && e.description === 'What we cooked',
      ),
      `Saving sends the edited entry, got ${JSON.stringify(saved)}`,
    )
    await page.screenshot({ path: resolve(dataDir, 'channel-guide-admin-saved.png') })
    await page.setViewportSize({ width: 390, height: 844 })
    assert(await noSideScroll(page), 'Welcome must not scroll sideways on a phone')
    await page.screenshot({ path: resolve(dataDir, 'channel-guide-admin-phone.png') })
    results.adminWelcome = 'passed'
    await context.close()
  }

  console.log('Checking the operator request tab')
  {
    const context = await browser.newContext({
      viewport: { width: 1280, height: 860 },
      serviceWorkers: 'block',
    })
    const state = createState()
    await install(context, state)
    const page = await context.newPage()
    page.on('pageerror', (error) => console.error('Browser error:', error.message))
    await page.goto(`${url}/channel/conv-general`, { waitUntil: 'domcontentloaded' })
    await page.locator('.channel-name-button').click()
    const settings = page.getByRole('dialog', { name: '#general settings' })
    await settings.getByRole('button', { name: 'Channel guide' }).click()
    await settings.getByText('Not listed').waitFor()
    await settings.getByLabel('Suggested group').fill('Games')
    await settings.getByLabel('Description').fill('Say hi')
    await settings.getByRole('button', { name: 'Ask to be listed' }).click()
    await settings.getByText('Waiting for a server owner').waitFor()
    const asked = state.calls.find((c) => c.method === 'PUT')?.body
    assert(
      asked?.group === 'Games' && asked.description === 'Say hi',
      `The request carries group and description, got ${JSON.stringify(asked)}`,
    )
    await page.screenshot({ path: resolve(dataDir, 'channel-guide-operator.png') })
    await settings.getByRole('button', { name: 'Withdraw request' }).click()
    await settings.getByText('Not listed').waitFor()
    assert(
      state.calls.some((c) => c.method === 'DELETE'),
      'Withdrawing calls the API',
    )
    results.operatorRequest = 'passed'
    await context.close()
  }

  console.log(JSON.stringify({ ok: true, results }, null, 2))
} finally {
  await browser.close()
}
