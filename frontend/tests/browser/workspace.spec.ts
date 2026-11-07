import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { mkdir } from 'node:fs/promises'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('renders the workspace and echoes an actual WebSocket message', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('hi')
  await input.press('Enter')
  await expect(page.locator('.message-text').filter({ hasText: /^hi$/ })).toHaveCount(2)
  await expect(page.locator('.is-pending')).toHaveCount(0)
  expect(errors).toEqual([])
})

test('typing from the workspace focuses the composer without losing the first key or stealing search input', async ({
  page,
}) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('draft ')
  await page.locator('.timeline-scroll').evaluate((element) => {
    ;(element as HTMLElement).focus()
    window.getSelection()?.removeAllRanges()
  })
  await page.keyboard.type('hello')
  await expect(input).toBeFocused()
  await expect(input).toHaveValue('draft hello')
  await page.getByRole('button', { name: 'Search messages', exact: true }).click()
  const search = page.getByRole('textbox', { name: 'Search workspace' })
  await search.fill('coffee')
  await page.keyboard.type(' break')
  await expect(search).toHaveValue('coffee break')
  await expect(input).toHaveValue('draft hello')
})

test('activity stays compact and expandable while synthetic history messages are hidden', async ({
  page,
}) => {
  const bootstrap = await (await page.request.get('/api/v1/bootstrap')).json()
  const actor = bootstrap.members.find((member: { id: string }) => member.id !== 'you')
  await page.route('**/api/v1/conversations/general/messages*', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    const base = { conversationId: 'general', senderId: actor.id, reactions: [] }
    const events = [
      { type: 'join' },
      { type: 'part', text: 'see you' },
      { type: 'kick', targetId: 'you', text: 'test reason' },
      { type: 'mode', text: '+b *!*@blocked.example' },
      { type: 'invite', targetId: 'you', text: '#lounge' },
    ]
    body.items.push(
      ...events.map((event, i) => ({
        ...base,
        id: `compact-${i}`,
        text: '',
        kind: 'event',
        createdAt: new Date(Date.now() + i).toISOString(),
        event: { actorId: actor.id, ...event },
      })),
    )
    body.items.push({
      ...base,
      id: 'system-history',
      text: 'HistServ replay noise',
      kind: 'system',
      createdAt: new Date().toISOString(),
    })
    await route.fulfill({ response, json: body })
  })
  await page.reload()
  const summary = page.locator('.event-summary').last()
  await expect(summary).toContainText('more')
  await summary.click()
  const details = page.locator('.event-list').last()
  await expect(details).toContainText('banned *!*@blocked.example')
  await expect(details).toContainText('invited')
  await expect(details).toContainText('#lounge')
  await expect(page.getByText('HistServ replay noise', { exact: true })).toHaveCount(0)
  await expect(page.locator('.event-row .avatar')).toHaveCount(0)
  await summary.click()
  const row = await summary.locator('..').boundingBox()
  expect(row!.height).toBeLessThan(30)
})

test('keeps drafts per conversation, supports replies, reactions and profiles', async ({
  page,
}) => {
  await page.getByRole('combobox', { name: 'Message #general' }).fill('a draft for general')
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeEnabled()
  await page
    .locator('.conversation-link')
    .filter({ hasText: /^general$/ })
    .click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toHaveValue(
    'a draft for general',
  )
  const last = page.locator('.message-row').last()
  await last.hover()
  await last.getByRole('button', { name: 'Reply to message' }).click()
  await expect(page.locator('.composer-reply')).toContainText('Replying to')
  await page.getByRole('combobox', { name: 'Message #general' }).fill('reply test')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(page.locator('.message-text').filter({ hasText: /^reply test$/ })).toHaveCount(2)
  await page.locator('.member-button').filter({ hasText: 'Mika' }).click()
  await expect(page.getByRole('dialog')).toContainText('Keeping the lights on')
  await page.getByRole('button', { name: 'Close dialog' }).click()
})

test('searches messages, joins channels and changes appearance', async ({ page }) => {
  await page.getByRole('button', { name: 'Search messages', exact: true }).click()
  await page.getByRole('textbox', { name: 'Search workspace' }).fill('coffee')
  await page.getByRole('tab', { name: /Messages/ }).click()
  await expect(page.locator('.search-message')).toHaveCount(1)
  await page.locator('.search-message').click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.getByRole('button', { name: 'Join a channel', exact: true }).click()
  await page.getByRole('textbox', { name: 'Channel name' }).fill('weekend-plans')
  await page.getByRole('button', { name: 'Join', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #weekend-plans' })).toBeEnabled()
  await expect(page.getByText('This is the beginning of #weekend-plans.')).toBeVisible()
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Light', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await page.getByRole('radio', { name: /^Compact/ }).click()
  await expect(page.locator('html')).toHaveAttribute('data-layout', 'compact')
  await page.getByRole('radio', { name: /^Tokyo Night/ }).click()
  await page.reload()
  await expect(page.locator('html')).toHaveAttribute('data-layout', 'compact')
  expect(
    await page.evaluate(() =>
      getComputedStyle(document.documentElement).getPropertyValue('--main'),
    ),
  ).toBe('#e1e2e7')
})

test('reconnects after offline mode without losing the draft', async ({ page, context }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('still here')
  await context.setOffline(true)
  await expect(page.locator('.connection-banner')).toContainText('offline')
  await expect(input).toBeDisabled()
  await context.setOffline(false)
  await expect(input).toBeEnabled()
  await expect(input).toHaveValue('still here')
  await input.press('Enter')
  await expect(page.locator('.message-text').filter({ hasText: /^still here$/ })).toHaveCount(2)
})

test('recovers from bootstrap failure and persists profile changes', async ({ page }) => {
  await page.route('**/api/v1/bootstrap', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: JSON.stringify({ statusMessage: 'Temporarily unavailable' }),
    }),
  )
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Couldn’t load your workspace' })).toBeVisible()
  await page.unroute('**/api/v1/bootstrap')
  await page.getByRole('button', { name: 'Try again', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Profile', exact: true }).click()
  await page.getByRole('textbox', { name: 'Display name', exact: true }).fill('A different name')
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page.reload()
  await expect(page.locator('.self-profile')).toContainText('A different name')
})

test('adds a demo network and sends in its new channel', async ({ page }) => {
  await page.getByRole('button', { name: 'Add a network', exact: true }).click()
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('My place')
  await page.getByRole('textbox', { name: 'Hostname' }).fill('irc.example.org')
  await page.getByRole('button', { name: 'Add network', exact: true }).click()
  await expect(page.locator('.network-heading')).toContainText('My place')
  await page.getByRole('combobox', { name: 'Message #general' }).fill('new place')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(page.locator('.message-text').filter({ hasText: /^new place$/ })).toHaveCount(2)
})

test('saves profile images and name colors, then restores automatic coloring', async ({ page }) => {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Profile', exact: true }).click()
  await page.getByLabel('Upload profile image').setInputFiles('public/avatars/jules.jpg')
  await expect(page.locator('.profile-avatar .avatar-image')).toBeVisible()
  await mkdir('.impeccable/review', { recursive: true })
  await page.screenshot({
    path: '.impeccable/review/profile-editor.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.getByLabel('Custom', { exact: true }).check()
  await page.getByLabel('Custom name color').fill('#ee88cc')
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(page.locator('.self-profile .avatar-image')).toBeVisible()
  await page.reload()
  await expect(page.locator('.self-profile .avatar-image')).toBeVisible()
  await page.getByRole('combobox', { name: 'Message #general' }).fill('profile check')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(
    page
      .locator('.message-row')
      .filter({ has: page.locator('.message-text').filter({ hasText: /^profile check$/ }) })
      .first()
      .locator('.author-name'),
  ).toHaveCSS('color', 'rgb(238, 136, 204)')
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Profile', exact: true }).click()
  await page.getByLabel('Automatic', { exact: true }).check()
  await page.getByRole('button', { name: 'Remove image', exact: true }).click()
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(page.locator('.self-profile .avatar-image')).toHaveCount(0)
})

test('composer stays on one row, grows for multiline text, and sends on mobile', async ({
  page,
}) => {
  await expect(page.getByText('A little corner of the internet.', { exact: true })).toHaveCount(0)
  await expect(page.getByText('Explore channels', { exact: true })).toHaveCount(0)
  const composer = page.locator('.composer')
  const single = (await composer.boundingBox())!.height
  expect(single).toBeLessThan(56)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('first line')
  await input.press('Shift+Enter')
  await input.pressSequentially('second line')
  expect((await composer.boundingBox())!.height).toBeGreaterThan(single + 10)
  await expect(input).toHaveValue('first line\nsecond line')
  await input.press('Enter')
  await expect(
    page.locator('.message-text').filter({ hasText: 'first line\nsecond line' }),
  ).toHaveCount(2)
  await page.setViewportSize({ width: 390, height: 844 })
  await input.fill('hello from mobile')
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(
    page.locator('.message-text').filter({ hasText: /^hello from mobile$/ }),
  ).toHaveCount(2)
})

test('captures desktop, settings, light, mobile and empty states', async ({ page }) => {
  await mkdir('.impeccable/review', { recursive: true })
  await page.evaluate(() => document.fonts.ready)
  await page.locator('.timeline-scroll').evaluate((el) => (el.scrollTop = el.scrollHeight))
  await page.screenshot({
    path: '.impeccable/review/desktop.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.screenshot({
    path: '.impeccable/review/settings.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.getByRole('button', { name: 'Light', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page.screenshot({
    path: '.impeccable/review/light.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.locator('.timeline-scroll').evaluate((el) => (el.scrollTop = el.scrollHeight))
  await page.screenshot({
    path: '.impeccable/review/mobile.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.getByRole('button', { name: 'Open conversations' }).click()
  await page.screenshot({
    path: '.impeccable/review/mobile-navigation.png',
    fullPage: true,
    animations: 'disabled',
  })
  await page.locator('.conversation-link').filter({ hasText: 'off-topic' }).click()
  await expect(page.getByRole('combobox', { name: 'Message #off-topic' })).toBeVisible()
  await expect(page.locator('.timeline-skeleton')).toHaveCount(0)
  await expect(page.getByText('This is the beginning of #off-topic.')).toBeVisible()
  await page.screenshot({
    path: '.impeccable/review/mobile-empty.png',
    fullPage: true,
    animations: 'disabled',
  })
  expect(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth)).toBe(
    false,
  )
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Dark', exact: true }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page
    .locator('.conversation-link')
    .filter({ hasText: /^general$/ })
    .click()
  await expect(page.locator('.timeline-skeleton')).toHaveCount(0)
  await page.setViewportSize({ width: 390, height: 844 })
  await page.locator('.timeline-scroll').evaluate((el) => (el.scrollTop = el.scrollHeight))
  await page.screenshot({
    path: '.impeccable/review/mobile-dark.png',
    fullPage: true,
    animations: 'disabled',
  })
})

test('has no serious accessibility violations on the workspace or settings', async ({ page }) => {
  const initial = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
    .analyze()
  expect(
    initial.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) })),
  ).toEqual([])
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  const settings = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
    .analyze()
  expect(
    settings.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) })),
  ).toEqual([])
  await page.getByRole('button', { name: 'Profile', exact: true }).click()
  const profile = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
    .analyze()
  expect(
    profile.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) })),
  ).toEqual([])
})

test('resizes the side panels and keeps their widths after reload', async ({ page }) => {
  const handle = page.getByRole('separator', { name: 'Resize conversation list' })
  const box = (await handle.boundingBox())!
  await page.mouse.move(box.x + 3, box.y + 200)
  await page.mouse.down()
  await page.mouse.move(box.x + 63, box.y + 200, { steps: 4 })
  await page.mouse.up()
  const width = (await page.locator('.conversation-sidebar').boundingBox())!.width
  expect(width).toBeGreaterThan(290)
  await page.getByRole('separator', { name: 'Resize member list' }).focus()
  await page.keyboard.press('ArrowLeft')
  await page.keyboard.press('ArrowLeft')
  const members = (await page.locator('.member-sidebar').boundingBox())!.width
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  expect((await page.locator('.conversation-sidebar').boundingBox())!.width).toBe(width)
  expect((await page.locator('.member-sidebar').boundingBox())!.width).toBe(members)
})

test('autocompletes emoji in the composer and converts typed shortcodes', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.click()
  await input.pressSequentially('nice :jo')
  await expect(page.getByRole('listbox', { name: 'Emoji matching :jo' })).toBeVisible()
  await expect(page.getByRole('option').first()).toContainText(':joy:')
  await page.keyboard.press('Enter')
  await expect(input).toHaveValue('nice 😂')
  await input.pressSequentially(' :fire:')
  await expect(input).toHaveValue('nice 😂 🔥')
  await input.pressSequentially(' :he')
  await page.keyboard.press('Escape')
  await expect(page.getByRole('listbox')).toHaveCount(0)
})

test('autocompletes mentions and highlights messages that mention you', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.click()
  await input.pressSequentially('hey @ec')
  await expect(page.getByRole('option')).toHaveCount(1)
  await page.keyboard.press('Tab')
  await expect(input).toHaveValue('hey @echo ')
  await input.fill('@karbowiak are you there?')
  await page.keyboard.press('Escape')
  await input.press('Enter')
  // Echo repeats the text, so the incoming copy mentions us.
  await expect(page.locator('.message-row.is-mention')).toHaveCount(1)
  await expect(page.locator('.inline-mention.is-self')).toHaveCount(2)
})

test('signs out and back in with the account key', async ({ page }) => {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  await page.getByRole('button', { name: 'Create new key' }).click()
  await page.getByRole('button', { name: 'Create new key' }).last().click()
  const key = (await page.locator('.account-key').textContent())!.trim()
  await page.getByRole('button', { name: 'Sign out' }).click()
  await page.getByRole('button', { name: 'I have a key' }).click()
  await page.getByLabel('Account key').fill(key)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})

test('runs slash commands and shows replies in the server window', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('/')
  await expect(page.getByRole('listbox', { name: 'Commands' })).toBeVisible()
  await input.fill('/me waves')
  await page.keyboard.press('Escape')
  await input.press('Enter')
  await expect(page.locator('.message-row.is-action').filter({ hasText: 'waves' })).toHaveCount(2)
  await input.fill('/whois mika')
  await page.keyboard.press('Escape')
  await input.press('Enter')
  await expect(page.getByRole('log', { name: /server messages/ })).toContainText(
    'mika is logged in as mika',
  )
  const raw = page.getByRole('combobox', { name: /Command to/ })
  await raw.fill('NOSUCH thing')
  await raw.press('Enter')
  await expect(page.locator('.server-line.is-error')).toContainText('Unknown command')
})

test('moderates from the member context menu when you are an operator', async ({ page }) => {
  await page.locator('.member-button').filter({ hasText: 'Noah' }).click({ button: 'right' })
  const menu = page.getByRole('menu')
  await expect(menu).toContainText('Make operator')
  await menu.getByRole('menuitem', { name: 'Give voice' }).click()
  await expect(
    page.locator('.member-button').filter({ hasText: 'Noah' }).locator('.member-prefix'),
  ).toHaveText('+')
  await expect(page.locator('.event-row').filter({ hasText: '+v noah' })).toHaveCount(1)
  // In #development you have no prefix, so moderation is not offered.
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await page.locator('.member-button').filter({ hasText: 'Noah' }).click({ button: 'right' })
  await expect(page.getByRole('menu')).not.toContainText('Make operator')
})

test('collapses join and part noise and attaches uploads', async ({ page }) => {
  await expect(page.locator('.event-summary').first()).toContainText('ada joined')
  await page.locator('input[type=file][multiple]').setInputFiles('public/avatars/noah.jpg')
  await expect(page.locator('.upload-item')).toHaveCount(1)
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  await expect(page.locator('.message-image img[src*="/api/v1/uploads/"]')).toHaveCount(2)
})

test('channel settings show modes and lists, and operators can change them', async ({ page }) => {
  await page.locator('.channel-name-button').click()
  const dialog = page.getByRole('dialog', { name: '#general settings' })
  await expect(dialog).toContainText('You’re an operator here')
  await dialog.getByRole('button', { name: 'Modes', exact: true }).click()
  await dialog.getByRole('switch', { name: /\+m Moderated/ }).click()
  await expect(dialog.getByRole('switch', { name: /\+m Moderated/ })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await dialog.getByRole('button', { name: 'Bans', exact: true }).click()
  await expect(dialog.locator('.mask-list')).toContainText('*!*@spam.example')
  await dialog.getByLabel('Add ban').fill('troll')
  await dialog.getByRole('button', { name: 'Add ban' }).click()
  await expect(dialog.locator('.mask-list')).toContainText('troll!*@*')
  await page.keyboard.press('Escape')
  // Without a prefix in #development, everything is read-only.
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await page.locator('.channel-name-button').click()
  await expect(page.getByRole('dialog')).toContainText('Read-only')
})

test('highlight words and ignore apply to the timeline', async ({ page }) => {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Notifications', exact: true }).click()
  await page.getByLabel('Add a highlight word').fill('coffee')
  await page.getByLabel('Add a highlight word').press('Enter')
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(
    page.locator('.message-row.is-mention').filter({ hasText: 'Coffee is on' }),
  ).toHaveCount(1)
  await page.locator('.member-button').filter({ hasText: 'Mika' }).click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Ignore' }).click()
  await expect(page.locator('.message-row').filter({ hasText: 'Coffee is on' })).toHaveCount(0)
})

test('the quick switcher ranks unread first and navigates with arrows', async ({ page }) => {
  await page.keyboard.press('Control+k')
  const first = page.locator('.search-conversation').first()
  await expect(first).toContainText('development')
  await expect(first).toHaveClass(/is-selected/)
  await page.keyboard.press('ArrowDown')
  await expect(page.locator('.search-conversation').nth(1)).toHaveClass(/is-selected/)
  await page.keyboard.press('ArrowUp')
  await page.keyboard.press('Enter')
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeVisible()
})

test('autocompletes channels and opens channel mentions', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.click()
  await input.pressSequentially('try #de')
  await expect(page.getByRole('listbox', { name: 'Channels matching #de' })).toBeVisible()
  await expect(page.getByRole('option').first()).toContainText('development')
  await page.keyboard.press('Tab')
  await expect(input).toHaveValue('try #development ')
  // Channels you haven't joined come from the network's channel list.
  await input.fill('')
  await input.pressSequentially('#pyth')
  await expect(page.getByRole('option').first()).toContainText('1,502 users')
  await input.fill('see #development.')
  await page.keyboard.press('Escape')
  await input.press('Enter')
  await page.locator('.inline-channel').filter({ hasText: '#development' }).last().click()
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeVisible()
})

test('conversations are in the URL and browser history moves between them', async ({ page }) => {
  const hash = () => page.evaluate(() => decodeURIComponent(location.hash))
  await expect.poll(hash).toBe('#libera-chat/#general')
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  await expect.poll(hash).toBe('#libera-chat/#development')
  await page.locator('.conversation-link').filter({ hasText: 'Jules' }).click()
  await expect.poll(hash).toBe('#libera-chat/@jules')
  await page.goBack()
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeVisible()
  await page.goBack()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeVisible()
  await page.goForward()
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeVisible()
  // Deep links open the conversation directly.
  await page.goto('/#libera-chat/%23music')
  await expect(page.getByRole('combobox', { name: 'Message #music' })).toBeVisible()
  // A link to a channel you're not in joins (and on IRC, creates) it.
  await page.goto('/#libera-chat/%23derp')
  await expect(page.getByRole('combobox', { name: 'Message #derp' })).toBeVisible()
  await expect(page.locator('.conversation-link').filter({ hasText: 'derp' })).toHaveCount(1)
})

test('copying a stretch of the timeline gives a plain log without controls', async ({ page }) => {
  await expect(page.locator('.message-text').filter({ hasText: 'Finally giving' })).toHaveCount(1)
  // Selects from the start of `from` to the end of `to` and copies, reporting what was written.
  const copy = (from: string, to: string, fromOffset = 0, toOffset?: number) =>
    page.evaluate(
      ({ from, to, fromOffset, toOffset }) => {
        // `selector` or `selector|text`: the first match whose text contains `text`.
        const node = (target: string) => {
          const [selector, has = ''] = target.split('|')
          const el = [...document.querySelectorAll(selector!)].find((e) =>
            e.textContent?.includes(has),
          )!
          const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT)
          const texts: Text[] = []
          while (walker.nextNode())
            if ((walker.currentNode as Text).data.trim()) texts.push(walker.currentNode as Text)
          return texts
        }
        const start = node(from)[0]!
        const end = node(to).at(-1)!
        const range = document.createRange()
        range.setStart(start, fromOffset)
        range.setEnd(end, toOffset ?? end.length)
        getSelection()!.removeAllRanges()
        getSelection()!.addRange(range)
        let copied = { prevented: false, text: '' }
        const listen = (e: ClipboardEvent) =>
          (copied = { prevented: e.defaultPrevented, text: e.clipboardData!.getData('text/plain') })
        window.addEventListener('copy', listen)
        document.execCommand('copy')
        window.removeEventListener('copy', listen)
        return copied
      },
      { from, to, fromOffset, toOffset },
    )
  const text = (snippet: string) => `.message-text|${snippet}`

  const across = await copy(text('Morning, everyone'), text('Finally giving'), 3)
  expect(across.prevented).toBe(true)
  const lines = across.text.split('\n')
  expect(lines).toHaveLength(3)
  expect(lines[0]).toMatch(/^--- .*\d{4}.* ---$/)
  expect(lines[1]).toMatch(/^\[[^\]]+\] <mika> Morning, everyone! Coffee is on\./)
  expect(lines[2]).toMatch(/^\[[^\]]+\] <jules> Finally giving my desk/)
  expect(across.text).not.toMatch(/👍|❤️|😂|\n\n/)

  // Inside one message, the browser copies exactly the selected text.
  const inside = await copy(text('Morning, everyone'), text('Morning, everyone'), 0, 7)
  expect(inside.prevented).toBe(false)

  // So do a timestamp and a date divider on their own.
  expect((await copy('.message-meta time', '.message-meta time')).prevented).toBe(false)
  expect((await copy('.date-divider', '.date-divider')).prevented).toBe(false)

  // An expanded activity run copies only the lines that were selected.
  await page.locator('.event-summary').first().click()
  const list = '.event-list li'
  const events = await copy(`${list}:nth-child(1)`, `${list}:nth-child(2)`)
  expect(events.prevented).toBe(true)
  expect(events.text.split('\n').slice(1)).toEqual([
    expect.stringMatching(/\*\*\* ada joined$/),
    expect.stringMatching(/\*\*\* lin left \(Leaving\)$/),
  ])
})

test('copying a selection longer than the rendered rows copies every selected message', async ({
  page,
}) => {
  // #general gets 150 earlier messages, far more than the virtualizer renders at once.
  await page.route('**/api/v1/conversations/general/messages**', async (route) => {
    const response = await route.fetch()
    const page = await response.json()
    const template = page.items.find((m: { kind: string }) => m.kind === 'message')
    const start = new Date(page.items[0].createdAt).getTime() - 3_600_000
    const filler = Array.from({ length: 150 }, (_, i) => ({
      ...template,
      id: `filler-${i}`,
      clientId: undefined,
      text: `filler line ${i}`,
      reactions: [],
      replyTo: undefined,
      attachment: undefined,
      gif: undefined,
      createdAt: new Date(start + i * 1000).toISOString(),
    }))
    await route.fulfill({ response, json: { items: [...filler, ...page.items], nextCursor: null } })
  })
  await page.reload()
  const scroller = page.locator('.timeline-scroll')
  await expect(page.locator('.message-text').filter({ hasText: 'Finally giving' })).toHaveCount(1)
  await scroller.evaluate((el) => (el.scrollTop = 0))
  await expect(page.locator('.message-text').filter({ hasText: /^filler line 0$/ })).toHaveCount(1)

  // Click into the first message, scroll to the latest one and shift-click its end.
  await page.evaluate(() => {
    const text = [...document.querySelectorAll('.message-text')].find(
      (el) => el.textContent === 'filler line 0',
    )!.firstChild!
    getSelection()!.collapse(text, 0)
  })
  await scroller.evaluate((el) => (el.scrollTop = el.scrollHeight))
  await expect(page.locator('.message-text').filter({ hasText: 'Finally giving' })).toHaveCount(1)
  // Scrolled away, the first message is no longer on screen.
  await expect
    .poll(() =>
      page.evaluate(() => {
        const text = [...document.querySelectorAll('.message-text')].find(
          (el) => el.textContent === 'filler line 0',
        )
        const box = document.querySelector('.timeline-scroll')!.getBoundingClientRect()
        return !text || text.getBoundingClientRect().bottom < box.top
      }),
    )
    .toBe(true)
  // Most of the selection is not rendered.
  expect(await page.locator('.virtual-row').count()).toBeLessThan(100)
  const copied = await page.evaluate(() => {
    const texts = [...document.querySelectorAll('.message-text')]
    const last = texts.at(-1)!
    const walker = document.createTreeWalker(last, NodeFilter.SHOW_TEXT)
    let end: Text | undefined
    while (walker.nextNode()) end = walker.currentNode as Text
    getSelection()!.extend(end!, end!.length)
    let text = ''
    const listen = (e: ClipboardEvent) => (text = e.clipboardData!.getData('text/plain'))
    window.addEventListener('copy', listen)
    document.execCommand('copy')
    window.removeEventListener('copy', listen)
    return text
  })
  const lines = copied.split('\n').filter((line) => !line.startsWith('--- '))
  expect(lines[0]).toMatch(/<\w+> filler line 0$/)
  for (let i = 0; i < 150; i++) expect(copied).toContain(`> filler line ${i}\n`)
  expect(lines.at(-1)).toMatch(/<echo> Make yourself at home/)
})
