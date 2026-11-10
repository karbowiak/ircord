import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'

const url = process.env.IRCORD_UI_URL ?? 'http://127.0.0.1:3000'
const dataDir = resolve(process.cwd(), '..', '.data')
mkdirSync(dataDir, { recursive: true })

function assert(condition, message) {
  if (!condition) throw new Error(message)
}

function makeBootstrap({
  demo = false,
  nick = 'newbie',
  realname = 'Old Real',
  displayName = 'Newbie Explicit',
  bio = 'Hello world',
  networks,
} = {}) {
  return {
    ...(demo ? { demo: true } : {}),
    cursor: 1,
    capabilities: { embeds: false },
    user: {
      id: 'you',
      nick,
      displayName,
      bio,
      status: 'online',
    },
    networks: networks ?? [
      {
        id: 'ircord',
        name: 'IRCord',
        initials: 'IR',
        color: 'mint',
        status: 'connected',
        host: 'irc.ircord.com',
        port: 6697,
        tls: true,
        nick,
        realname,
        profile: { id: 'you', nick, displayName, nameSource: 'profile', bio, status: 'online' },
        account: nick,
        auth: { method: 'sasl-plain', account: nick, passwordSet: true },
        autoJoin: ['#general'],
        perform: [],
        capabilities: ['sasl', 'setname'],
        isupport: { NETWORK: 'IRCord', NICKLEN: '30', CASEMAPPING: 'rfc1459' },
        features: {
          setname: true,
          metadata: ['display-name', 'avatar'],
          away: true,
          redact: true,
          edit: true,
          replies: true,
          reactions: true,
          typing: true,
          attract: true,
          readMarkers: true,
          history: true,
          messageLineBytes: 310,
        },
      },
    ],
    conversations: [
      {
        id: 'general',
        networkId: 'ircord',
        name: 'general',
        kind: 'channel',
        joined: true,
        topic: 'Welcome to IRCord',
        unreadCount: 0,
        mentionCount: 0,
        favorite: false,
        memberIds: ['you'],
      },
    ],
    members: [
      {
        id: 'you',
        nick,
        displayName,
        bio,
        status: 'online',
      },
    ],
  }
}

const json = (route, body, status = 200) =>
  route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })

async function installMocks(context, state) {
  await context.route('**/api/v1/**', async (route) => {
    const req = route.request()
    const { pathname } = new URL(req.url())
    const method = req.method()

    if (pathname === '/api/v1/auth/registration' && method === 'GET')
      return json(route, {
        registrationEnabled: true,
        registrationKeyRequired: Boolean(state.registrationKeyRequired),
      })
    if (pathname === '/api/v1/auth/session' && method === 'GET')
      return json(route, { account: state.account })
    if (pathname === '/api/v1/auth/username' && method === 'GET')
      return json(route, { available: true })
    if (pathname === '/api/v1/auth/onboarding/complete' && method === 'POST') {
      state.completeCalls.push(Date.now())
      if (state.failCompleteOnce) {
        state.failCompleteOnce = false
        return json(route, { detail: 'Temporary completion failure.' }, 502)
      }
      if (state.account) {
        const next = { ...state.account }
        delete next.onboardingNetworkId
        state.account = next
      }
      return route.fulfill({ status: 204, body: '' })
    }
    const profileMatch = /^\/api\/v1\/networks\/([^/]+)\/profile$/.exec(pathname)
    if (profileMatch && method === 'GET') {
      const network = state.bootstrap.networks.find((n) => n.id === profileMatch[1])
      network.profile ??= state.profileReads?.[profileMatch[1]]
      return json(route, network.profile)
    }
    if (profileMatch && method === 'PATCH') {
      const body = req.postDataJSON()
      state.profilePatches.push(body)
      state.profileTargets.push(profileMatch[1])
      if (state.profileGate) await state.profileGate
      if (state.failProfileOnce) {
        state.failProfileOnce = false
        return json(route, { detail: 'Profile save failed.' }, 502)
      }
      const profile = state.bootstrap.networks.find((n) => n.id === profileMatch[1]).profile
      if (body.displayName !== undefined) {
        profile.displayName = body.displayName.trim() || profile.nick
        profile.nameSource = body.displayName.trim() ? 'profile' : 'nick'
      }
      if (body.bio !== undefined) profile.bio = body.bio
      if (body.nameColor !== undefined) profile.nameColor = body.nameColor
      if (body.avatarUrl !== undefined) profile.avatarUrl = body.avatarUrl
      if (body.avatarDataUrl === null) delete profile.avatarUrl
      else if (typeof body.avatarDataUrl === 'string') profile.avatarUrl = body.avatarDataUrl
      return json(route, profile)
    }
    if (pathname === '/api/v1/networks' && method === 'POST') {
      const body = req.postDataJSON()
      state.networkPosts.push(body)
      return json(route, { id: 'added-net', ...body, status: 'connected' })
    }
    if (pathname.startsWith('/api/v1/networks/') && method === 'PATCH') {
      const id = pathname.split('/').pop()
      const body = req.postDataJSON()
      state.networkPatches.push({ id, body })
      if (state.failNetworkOnce) {
        state.failNetworkOnce = false
        return json(route, { detail: 'Network update failed. Try again.' }, 502)
      }
      const target = state.bootstrap.networks.find((n) => n.id === id)
      if (target) {
        if (typeof body.nick === 'string') target.nick = body.nick
        if (typeof body.realname === 'string') target.realname = body.realname
        if (body.auth?.password === '') target.auth.passwordSet = false
      }
      return json(route, target ?? { id, ...body })
    }
    if (pathname === '/api/v1/networks/check' && method === 'POST') {
      const body = req.postDataJSON()
      state.networkChecks.push(body)
      if (state.failNetworkCheck)
        return json(route, { reachable: false, detail: 'Connection refused by remote host.' })
      return json(route, { reachable: true, detail: 'Connected over TLS 1.3.' })
    }
    if (pathname === '/api/v1/bootstrap' && method === 'GET')
      return state.failBootstrap
        ? json(route, { detail: 'Offline' }, 503)
        : json(route, state.bootstrap)
    if (pathname === '/api/v1/search' && method === 'GET')
      return json(route, { items: state.searchResults ?? [], nextCursor: null })
    if (pathname === '/api/v1/saved-messages' && method === 'GET')
      return json(route, { items: state.savedResults ?? [], nextCursor: null })
    if (pathname.endsWith('/messages') && method === 'GET')
      return json(route, {
        items: state.messagePages?.[pathname.split('/')[4]] ?? [],
        nextCursor: null,
      })
    if (pathname.endsWith('/Connection') || pathname.endsWith('/connection'))
      return json(route, { networkId: 'ircord', latencyMs: 12 })
    if (pathname.endsWith('/files/provider')) return json(route, { available: false })
    if (pathname === '/api/v1/appearance' && method === 'GET')
      return json(route, {
        theme: 'midnight',
        mode: 'dark',
        font: 'geist',
        monoFont: 'geist-mono',
        density: 'comfortable',
        interfaceScale: 100,
        chatFontSize: 15,
        lineSpacing: 'normal',
        timeFormat: '24h',
        messageGrouping: 'cozy',
        coloredNicknames: true,
        reducedMotion: 'system',
        customThemes: [],
      })
    if (pathname === '/api/v1/client-preferences' && method === 'GET')
      return json(route, {
        events: 'collapse',
        spellcheck: true,
        enterToSend: true,
        mediaPreviews: true,
        autoPlayGifs: true,
        networkOrder: [],
        networkIcons: {},
        collapsedCategories: [],
        collapsedNetworks: [],
        sidebarOrder: {},
        shortcuts: {},
      })
    if (pathname === '/api/v1/notifications/settings' && method === 'GET')
      return json(route, {
        desktop: 'mentions',
        sound: 'mentions',
        flashTaskbar: true,
        highlights: [],
        ignored: [],
        quietHours: { enabled: false, start: '22:00', end: '08:00', timezone: 'UTC' },
        conversations: {},
      })
    if (pathname === '/api/v1/saved-message-ids' && method === 'GET')
      return json(route, { items: state.savedResults ?? [] })
    if (pathname === '/api/v1/inbox' && method === 'GET')
      return json(route, { items: [], unreadCount: 0, nextCursor: null })
    if (pathname === '/api/v1/library' && method === 'GET')
      return json(route, { gifs: [], stickers: [] })
    if (pathname === '/api/v1/emotes' && method === 'GET')
      return json(route, { version: 1, emotes: [] })
    return json(route, {})
  })
}

function createState(overrides = {}) {
  return {
    registrationKeyRequired: true,
    account: {
      id: 'acc-new',
      username: 'newbie',
      onboardingNetworkId: 'ircord',
      createdAt: '2026-11-10T20:00:00Z',
      lastSeenAt: '2026-11-10T20:00:00Z',
      expiresAt: '2027-02-08T20:00:00Z',
    },
    bootstrap: makeBootstrap(),
    completeCalls: [],
    profilePatches: [],
    profileTargets: [],
    networkPatches: [],
    networkPosts: [],
    networkChecks: [],
    failCompleteOnce: false,
    failProfileOnce: false,
    failNetworkOnce: false,
    failNetworkCheck: false,
    ...overrides,
  }
}

const browser = await chromium.launch()
const results = {}

try {
  // 1. Credentials-only signup UI check
  console.log('Checking signup')
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    const state = createState({ account: null, registrationKeyRequired: true })
    await installMocks(context, state)
    const page = await context.newPage()
    page.on('pageerror', (error) => console.error('Browser error:', error.message))
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.getByRole('button', { name: 'Create an account' }).click()
    await page.locator('#signup-username').waitFor({ state: 'visible' })
    await page.locator('#signup-password').waitFor({ state: 'visible' })
    await page.locator('#registration-key').waitFor({ state: 'visible' })
    assert(
      (await page.locator('#signup-display-name').count()) === 0,
      'Signup form must not include display name input',
    )
    assert(
      (await page.locator('.signup-picture').count()) === 0,
      'Signup form must not include inline profile picture picker',
    )
    results.credentialsOnlySignup = 'passed'
    await context.close()
  }

  // 2. Pending new-account fixture: prefilled welcome, interrupted reload, failed save + retry, no duplicate POST
  console.log('Checking welcome retry')
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    const state = createState()
    await installMocks(context, state)
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded' })

    const dialog = page.getByRole('dialog', { name: 'Set up your profile' })
    await dialog.waitFor({ state: 'visible' })

    // Verify prefill and focus
    assert(
      (await page.locator('#welcome-nick').inputValue()) === 'newbie',
      'Nickname should be prefilled from pending default network',
    )
    assert(
      (await page.locator('#welcome-display-name').inputValue()) === 'Newbie Explicit',
      'Display name should be prefilled with explicit existing value',
    )
    assert(
      (await page.locator('#welcome-realname').inputValue()) === 'Old Real',
      'Real name should be prefilled from pending default network',
    )
    assert(
      (await page.locator('#welcome-bio').inputValue()) === 'Hello world',
      'Bio should be prefilled from user profile',
    )
    assert(
      (await dialog.locator('#network-host, #network-port').count()) === 0,
      'Welcome dialog must not expose server/address/auth fields',
    )
    const focusedId = await page.evaluate(() => document.activeElement?.id ?? '')
    assert(focusedId === 'welcome-nick', `Expected #welcome-nick focused, got "${focusedId}"`)

    const desktopShot = resolve(dataDir, 'onboarding-ui-desktop.png')
    await page.screenshot({ path: desktopShot })
    results.desktopScreenshot = desktopShot

    // Interrupted reload before saving/skipping retains pending welcome
    await page.reload({ waitUntil: 'domcontentloaded' })
    await dialog.waitFor({ state: 'visible' })
    assert(state.completeCalls.length === 0, 'Reload must not complete onboarding')

    // Mobile viewport & scroll/keyboard usability check
    await page.setViewportSize({ width: 375, height: 667 })
    await dialog.waitFor({ state: 'visible' })
    await dialog.getByRole('button', { name: 'Save profile' }).scrollIntoViewIfNeeded()
    assert(
      await dialog.getByRole('button', { name: 'Save profile' }).isVisible(),
      'Mobile welcome dialog must scroll to primary action',
    )
    await page.locator('#welcome-nick').scrollIntoViewIfNeeded()
    const mobileShot = resolve(dataDir, 'onboarding-ui-mobile.png')
    await page.screenshot({ path: mobileShot })
    results.mobileScreenshot = mobileShot
    await page.setViewportSize({ width: 1280, height: 840 })

    // Edit both profile and network fields; fail network PATCH once to test error + retry
    await page.locator('#welcome-display-name').fill('Newbie Updated')
    await page.locator('#welcome-nick').fill('newbie_irc')
    await page.locator('#welcome-realname').fill('Newbie WHOIS')
    state.failNetworkOnce = true

    await dialog.getByRole('button', { name: 'Save profile' }).click()
    await dialog.locator('.form-error').waitFor({ state: 'visible' })
    assert(
      (await dialog.locator('.form-error').textContent())?.includes('Network update failed'),
      'Failed save should display inline error in welcome dialog',
    )
    assert(await dialog.isVisible(), 'Welcome dialog must remain open on save failure')
    assert(state.profilePatches.length === 1, 'Profile PATCH should have succeeded once')
    assert(state.networkPatches.length === 1, 'Network PATCH should have been attempted once')
    assert(
      state.completeCalls.length === 0,
      'completeOnboarding must not be called when a save fails',
    )

    // Retry: profile was already saved, so only network PATCH and completeOnboarding should run
    await dialog.getByRole('button', { name: 'Save profile' }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert(state.profilePatches.length === 1, 'Retry must not re-send already-saved profile PATCH')
    assert(state.networkPatches.length === 2, 'Retry should re-send failed network PATCH')
    assert(
      state.networkPatches[1].id === 'ircord' &&
        state.networkPatches[1].body.nick === 'newbie_irc' &&
        state.networkPatches[1].body.realname === 'Newbie WHOIS' &&
        !('auth' in state.networkPatches[1].body),
      'Network PATCH must target default network with only changed identity fields and never overwrite auth/passwords',
    )
    assert(state.networkPosts.length === 0, 'Welcome save must never POST a duplicate network')
    assert(state.completeCalls.length === 1, 'completeOnboarding should be called once after save')

    // Reload after completion: welcome never reappears
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('.workspace').waitFor({ state: 'visible' })
    assert((await dialog.count()) === 0, 'Completed welcome dialog must not reappear on reload')
    results.pendingSaveRetryAndPersistence = 'passed'
    await context.close()
  }

  // 3. Unchanged network identity avoids unnecessary PATCH /api/v1/networks/{id} reconnect; skip persists dismissal
  console.log('Checking welcome skip')
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    const state = createState()
    await installMocks(context, state)
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded' })

    const dialog = page.getByRole('dialog', { name: 'Set up your profile' })
    await dialog.waitFor({ state: 'visible' })

    // Change only display name; leave nick & realname unchanged
    await page.locator('#welcome-display-name').fill('Only Profile Changed')
    await dialog.getByRole('button', { name: 'Save profile' }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert(state.profilePatches.length === 1, 'Profile PATCH should run when display name changed')
    assert(
      state.networkPatches.length === 0,
      'Network PATCH must NOT run when nick and realname are unchanged',
    )
    assert(state.completeCalls.length === 1, 'completeOnboarding should run after profile save')

    // Now test explicit Skip on a fresh pending account
    state.account = { ...state.account, onboardingNetworkId: 'ircord' }
    state.profilePatches.length = 0
    state.networkPatches.length = 0
    state.completeCalls.length = 0
    await page.reload({ waitUntil: 'domcontentloaded' })
    await dialog.waitFor({ state: 'visible' })
    await dialog.getByRole('button', { name: 'Skip for now' }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert(state.profilePatches.length === 0, 'Skip must not modify profile')
    assert(state.networkPatches.length === 0, 'Skip must not modify network')
    assert(state.completeCalls.length === 1, 'Skip must persist completion')
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('.workspace').waitFor({ state: 'visible' })
    assert((await dialog.count()) === 0, 'Skipped welcome must not reappear after reload')
    results.unchangedIdentityAndSkip = 'passed'
    await context.close()
  }

  // 4. Completed/legacy, demo, empty-networks, +Add Network, and explicit irc:// link priority
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    // 4a. Demo mode with onboardingNetworkId set must never show welcome
    const state = createState({
      bootstrap: makeBootstrap({ demo: true }),
    })
    await installMocks(context, state)
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.locator('.workspace').waitFor({ state: 'visible' })
    assert(
      (await page.getByRole('dialog', { name: 'Set up your profile' }).count()) === 0,
      'Demo mode must never show welcome dialog',
    )

    // 4b. Empty networks without onboardingNetworkId: no auto-dialog, timeline offers Add a network + Edit profile, and +Add Network opens standard Add network dialog
    await page.evaluate(() => {
      localStorage.clear()
      sessionStorage.clear()
    })
    state.account = {
      id: 'acc-legacy',
      username: 'veteran',
      createdAt: '2026-11-10T20:00:00Z',
      lastSeenAt: '2026-11-10T20:00:00Z',
      expiresAt: '2027-02-08T20:00:00Z',
    }
    state.bootstrap = makeBootstrap({ demo: false, networks: [] })
    state.bootstrap.conversations = []
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.locator('.conversation-pane').waitFor({ state: 'visible' })
    assert(
      (await page.getByRole('dialog').count()) === 0,
      'Empty networks must not auto-open a dialog',
    )
    await page.locator('.conversation-pane').getByRole('button', { name: 'Edit profile' }).waitFor({
      state: 'visible',
    })
    await page.locator('.conversation-pane').getByRole('button', { name: 'Add a network' }).click()
    const addDialog = page.getByRole('dialog', { name: 'Add a network' })
    await addDialog.waitFor({ state: 'visible' })
    assert(
      await addDialog.locator('#network-host').isVisible(),
      '+ Add a network must open full connection settings, not firstConnection quick setup',
    )
    await addDialog.getByRole('button', { name: 'Cancel' }).click()
    await addDialog.waitFor({ state: 'hidden' })

    // 4c. Explicit ?irc= link with pending onboardingNetworkId defers welcome until link dialog closes
    await page.evaluate(() => {
      localStorage.clear()
      sessionStorage.clear()
    })
    state.account = {
      ...state.account,
      onboardingNetworkId: 'ircord',
    }
    state.bootstrap = makeBootstrap()
    await page.goto(`${url}/?irc=${encodeURIComponent('ircs://irc.libera.chat/#linux')}`, {
      waitUntil: 'domcontentloaded',
    })
    await addDialog.waitFor({ state: 'visible' })
    assert(
      (await page.locator('#network-host').inputValue()) === 'irc.libera.chat',
      'Explicit irc:// link should prefill Add a network dialog first',
    )
    assert(
      (await page.getByRole('dialog', { name: 'Set up your profile' }).count()) === 0,
      'Welcome dialog must not stack over or hijack explicit irc:// link dialog',
    )
    await addDialog.getByRole('button', { name: 'Cancel' }).click()
    await page.getByRole('dialog', { name: 'Set up your profile' }).waitFor({ state: 'visible' })
    results.eligibilityAndLinkPriority = 'passed'
    await context.close()
  }

  // 5. Three reviewed NetworkDialog issues: Forget saved password, no Certificate option, and invalid hidden endpoint visibility
  console.log('Checking network settings')
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    const state = createState({
      account: {
        id: 'acc-net',
        username: 'netuser',
        createdAt: '2026-11-10T20:00:00Z',
        lastSeenAt: '2026-11-10T20:00:00Z',
        expiresAt: '2027-02-08T20:00:00Z',
      },
    })
    await installMocks(context, state)
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.locator('.workspace').waitFor({ state: 'visible' })

    // Open Settings -> Networks -> Edit IRCord
    await page.getByRole('button', { name: 'Settings' }).first().click()
    const settingsDialog = page.getByRole('dialog', { name: 'Settings' })
    await settingsDialog.waitFor({ state: 'visible' })
    await settingsDialog.locator('[data-section="networks"]').click()
    await settingsDialog.getByRole('button', { name: 'Edit' }).first().click()

    const editDialog = page.getByRole('dialog', { name: 'Edit IRCord' })
    await editDialog.waitFor({ state: 'visible' })

    // Issue 5.2: Certificate login method is not offered
    assert(
      (await editDialog.locator('.auth-methods').getByText('Certificate').count()) === 0,
      'Unsupported Certificate login method must not be offered',
    )

    // Issue 5.1: Forget saved password submits explicit clear (password: '') without HTML required blocking
    await editDialog.getByRole('button', { name: 'Forget saved password' }).click()
    await editDialog.getByRole('button', { name: 'Save changes' }).click()
    await editDialog.waitFor({ state: 'hidden' })
    assert(
      state.networkPatches.length === 1 && state.networkPatches[0].body.auth?.password === '',
      'Forget saved password must submit explicit empty password without HTML required blocking',
    )
    results.networkFormFixes = 'passed'
    await context.close()
  }

  // 6. Welcome edits the provisioned home profile even when another server is first.
  console.log('Checking welcome home server')
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    const bootstrap = makeBootstrap()
    const home = bootstrap.networks[0]
    bootstrap.networks.unshift({
      ...home,
      id: 'cfcord',
      name: 'CFCord',
      nick: 'cf-nick',
      profile: { ...home.profile, nick: 'cf-nick', displayName: 'Other server', bio: 'Other bio' },
    })
    const state = createState({ bootstrap })
    await installMocks(context, state)
    const page = await context.newPage()
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    const dialog = page.getByRole('dialog', { name: 'Set up your profile' })
    await dialog.waitFor({ timeout: 8000 }).catch(async (error) => {
      await page.screenshot({ path: resolve(dataDir, 'network-profile-welcome-debug.png') })
      throw error
    })
    assert(
      (await page.locator('#welcome-display-name').inputValue()) === 'Newbie Explicit',
      'Welcome must load the home profile',
    )
    await page.locator('#welcome-display-name').fill('Home only')
    await dialog.getByRole('button', { name: 'Save profile' }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert(state.profileTargets[0] === 'ircord', 'Welcome must save to the home server')
    assert(
      bootstrap.networks[0].profile.displayName === 'Other server',
      'Welcome must leave the other server unchanged',
    )
    results.welcomeHomeProfile = 'passed'
    await context.close()
  }

  // 7. Two isolated server profiles, including a save while the selector changes.
  console.log('Checking profile isolation')
  {
    const context = await browser.newContext({ viewport: { width: 1280, height: 840 } })
    const bootstrap = makeBootstrap()
    bootstrap.version = 1
    const home = bootstrap.networks[0]
    const other = {
      ...home,
      id: 'cfcord',
      name: 'CFCord',
      nick: 'cf-nick',
      profile: { ...home.profile, nick: 'cf-nick', displayName: 'CF original', bio: 'CF bio' },
    }
    const loadedProfile = other.profile
    delete other.profile
    bootstrap.networks.push(other)
    const homeConversation = bootstrap.conversations[0]
    const otherConversation = { ...homeConversation, id: 'cf-general', networkId: 'cfcord' }
    bootstrap.conversations.push(otherConversation)
    const message = (id, conversationId, text) => ({
      id,
      conversationId,
      senderId: 'you',
      text,
      createdAt: '2026-11-10T20:00:00Z',
      kind: 'message',
      reactions: [],
    })
    const homeMessage = message('home-message', 'general', 'Home message')
    const otherMessage = message('cf-message', 'cf-general', 'Other message')
    const otherReply = {
      ...message('cf-reply', 'cf-general', 'A reply'),
      replyTo: { id: 'cf-message', nick: 'cf-nick', text: 'Other message' },
    }
    const state = createState({
      account: { id: 'acc-isolation', username: 'newbie' },
      bootstrap,
      profileReads: { cfcord: loadedProfile },
      messagePages: { general: [homeMessage], 'cf-general': [otherMessage, otherReply] },
      searchResults: [
        { conversation: homeConversation, message: homeMessage },
        { conversation: otherConversation, message: otherMessage },
      ],
      savedResults: [
        {
          status: 'available',
          networkId: 'ircord',
          conversationId: 'general',
          messageId: homeMessage.id,
          savedAt: homeMessage.createdAt,
          conversation: homeConversation,
          message: homeMessage,
        },
        {
          status: 'available',
          networkId: 'cfcord',
          conversationId: 'cf-general',
          messageId: otherMessage.id,
          savedAt: otherMessage.createdAt,
          conversation: otherConversation,
          message: otherMessage,
        },
      ],
    })
    await context.routeWebSocket('**/api/v1/events**', (socket) => {
      if (state.socketOffline) socket.close()
      else socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: 1, sync: false }))
    })
    await installMocks(context, state)
    const page = await context.newPage()
    page.on('pageerror', (error) => console.error('Profile browser error:', error.stack))
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.locator('.workspace').waitFor({ state: 'visible' })
    await page.locator('.network-tab').filter({ hasText: 'IRCord' }).click()
    await page
      .locator('.message-row')
      .filter({ hasText: 'Home message' })
      .getByText('Newbie Explicit')
      .waitFor()
    await page.getByRole('button', { name: 'Settings' }).first().click()
    const settings = page.getByRole('dialog', { name: 'Settings' })
    await settings.locator('[data-section="profile"]').click()
    await page.screenshot({ path: resolve(dataDir, 'network-profile-debug.png') })
    const selector = page.locator('#profile-server')
    assert(
      (await selector.inputValue()) === 'ircord',
      'Profile selector should start on active server',
    )
    await selector.selectOption('cfcord')
    await page.getByText('Profile for CFCord').waitFor()
    assert(
      (await page.locator('#profile-name').inputValue()) === 'CF original',
      'CFCord profile should load',
    )
    await page.locator('#profile-name').fill('CF changed')
    await page.locator('#profile-bio').fill('Changed only on CFCord')
    await page.locator('#profile-color-custom').click()
    await page.locator('input[type="color"]').fill('#ab1234')
    await page.locator('.default-avatar-picker [role="radio"]').first().click()
    let release
    state.profileGate = new Promise((resolve) => {
      release = resolve
    })
    await settings.getByRole('button', { name: 'Save changes' }).click()
    await page.waitForFunction(() => document.querySelector('#profile-server') !== null)
    await selector.selectOption('ircord')
    await page.getByText('Profile for IRCord').waitFor()
    assert(
      (await page.locator('#profile-name').inputValue()) === 'Newbie Explicit',
      'Home draft must remain unchanged',
    )
    release()
    await page.waitForTimeout(250)
    assert(state.profileTargets[0] === 'cfcord', 'Save must target the original server')
    assert(
      home.profile.displayName === 'Newbie Explicit' && home.profile.bio === 'Hello world',
      'Home profile must not change',
    )
    assert(
      other.profile.displayName === 'CF changed' && other.profile.bio === 'Changed only on CFCord',
      'CFCord profile must save',
    )
    assert(
      other.profile.nameColor === '#ab1234' && !!other.profile.avatarUrl,
      `CFCord color and avatar must save: ${JSON.stringify(state.profilePatches[0])}`,
    )
    assert(
      (await settings.getByText('Profile saved').count()) === 0,
      'Save status must not appear on another server',
    )
    await selector.selectOption('cfcord')
    assert(
      (await page.locator('#profile-name').inputValue()) === 'CF changed',
      'Switching back must show the saved profile',
    )
    await page.locator('#profile-server').scrollIntoViewIfNeeded()
    await page.screenshot({ path: resolve(dataDir, 'network-profile-desktop.png') })
    await page.setViewportSize({ width: 390, height: 844 })
    await page.locator('#profile-server').scrollIntoViewIfNeeded()
    await page.screenshot({ path: resolve(dataDir, 'network-profile-mobile.png') })
    await page.setViewportSize({ width: 1280, height: 840 })
    await settings.getByRole('button', { name: 'Close' }).click()
    await page.locator('.network-tab').filter({ hasText: 'CFCord' }).click()
    await page.locator('.self-profile').getByText('CF changed').waitFor()
    await page
      .locator('.message-row')
      .filter({ hasText: 'Other message' })
      .filter({ hasNotText: 'A reply' })
      .getByText('CF changed')
      .waitFor()
    await page
      .locator('.message-row')
      .filter({ hasText: 'A reply' })
      .locator('.reply-nick')
      .getByText('cf-nick')
      .waitFor()
    await page.locator('.self-profile').click()
    await page
      .getByRole('dialog', { name: 'CF changed' })
      .getByText('Changed only on CFCord')
      .waitFor()
    await page
      .getByRole('dialog', { name: 'CF changed' })
      .getByRole('button', { name: 'Close' })
      .click()
    await page.locator('.network-tab').filter({ hasText: 'IRCord' }).click()
    await page.locator('.self-profile').getByText('Newbie Explicit').waitFor()
    await page.goto(`${url}/saved`)
    await page
      .locator('.search-message')
      .filter({ hasText: 'Home message' })
      .getByText('Newbie Explicit')
      .waitFor()
    await page
      .locator('.search-message')
      .filter({ hasText: 'Other message' })
      .getByText('CF changed')
      .waitFor()
    await page.goto(`${url}/search`)
    await page.getByRole('combobox', { name: 'Search workspace' }).fill('message')
    await page.getByRole('tab', { name: /^Messages/ }).click()
    await page.waitForTimeout(1000)
    await page.screenshot({ path: resolve(dataDir, 'network-profile-search-debug.png') })
    const homeResult = page.locator('.search-message').filter({ hasText: 'Home message' })
    const otherResult = page.locator('.search-message').filter({ hasText: 'Other message' })
    await homeResult.getByText('Newbie Explicit').waitFor({ timeout: 5000 })
    await otherResult.getByText('CF changed').waitFor({ timeout: 5000 })
    await page.evaluate(
      ({ bootstrap, pages }) => {
        localStorage.setItem(
          'ircord.snapshot.acc-isolation',
          JSON.stringify({
            version: 3,
            accountId: 'acc-isolation',
            savedAt: new Date().toISOString(),
            bootstrap,
            messages: Object.fromEntries(
              Object.entries(pages).map(([id, items]) => [id, { items, nextCursor: null }]),
            ),
          }),
        )
      },
      { bootstrap: state.bootstrap, pages: state.messagePages },
    )
    state.failBootstrap = true
    state.socketOffline = true
    await page.goto(url, { waitUntil: 'domcontentloaded' })
    await page.locator('.network-tab').filter({ hasText: 'CFCord' }).click()
    await page.locator('.self-profile').getByText('CF changed').waitFor()
    await page.locator('.network-tab').filter({ hasText: 'IRCord' }).click()
    await page.locator('.self-profile').getByText('Newbie Explicit').waitFor()
    results.offlineSnapshotIsolation = 'passed'
    results.networkProfileIsolation = 'passed'
    await context.close()
  }

  console.log(JSON.stringify({ ok: true, results }, null, 2))
} finally {
  await browser.close()
}
