import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { readFile } from 'node:fs/promises'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const violations = async (page: Page) =>
  (
    await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze()
  ).violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }))
const link = (page: Page, name: string) =>
  page.locator('.conversation-sidebar .conversation-link').filter({ hasText: name })

test('the shortcut overlay opens with ? and is accessible', async ({ page }) => {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur())
  await page.keyboard.press('?')
  const dialog = page.getByRole('dialog', { name: 'Keyboard shortcuts' })
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText('Next unread')
  expect(await violations(page)).toEqual([])
  await page.keyboard.press('Escape')
  await expect(dialog).toBeHidden()
  // Typing “?” in the composer is just text.
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('why?')
  await expect(dialog).toBeHidden()
})

test('unread only filter, mark all read and next-unread navigation', async ({ page }) => {
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur())
  await page.keyboard.press('Alt+Shift+ArrowDown')
  // Mentions come first.
  await expect(page.getByRole('combobox', { name: 'Message #development' })).toBeVisible()
  await page.getByRole('button', { name: 'Unread only' }).click()
  await expect(link(page, 'design')).toHaveCount(0)
  await expect(link(page, 'development')).toHaveCount(1)
  await page.getByRole('button', { name: 'Mark all read' }).click()
  await expect(
    page.locator('.conversation-sidebar .unread-dot, .conversation-sidebar .mention-badge'),
  ).toHaveCount(0)
  // Only the open conversation stays listed.
  await expect(page.locator('.sidebar-section .conversation-link')).toHaveCount(1)
  await page.reload()
  await expect(page.getByRole('button', { name: 'Unread only' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await page.getByRole('button', { name: 'Unread only' }).click()
  await expect(link(page, 'design')).toHaveCount(1)
})

test('exports a conversation as a plain-text log', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('exported line')
  await input.press('Enter')
  await expect(page.locator('.is-pending')).toHaveCount(0)
  const download = page.waitForEvent('download')
  await link(page, 'general').click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Export log' }).click()
  const file = await download
  expect(file.suggestedFilename()).toMatch(/^libera-chat_general_\d{4}-\d{2}-\d{2}\.log$/)
  const text = await readFile((await file.path())!, 'utf8')
  expect(text).toMatch(/\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}\] <[^>]+> exported line/)
})

test('irc:// links open known networks or prefill a new one', async ({ page }) => {
  await page.goto('/?irc=' + encodeURIComponent('ircs://irc.libera.chat/rust'))
  await expect(page.getByRole('combobox', { name: 'Message #rust' })).toBeVisible()
  await expect.poll(() => page.evaluate(() => location.search)).toBe('')
  await page.goto('/?irc=' + encodeURIComponent('irc://irc.newnet.example:6668/lobby'))
  const dialog = page.getByRole('dialog', { name: 'Add a network' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('Hostname')).toHaveValue('irc.newnet.example')
  await expect(dialog.getByLabel('Port')).toHaveValue('6668')
  expect(await violations(page)).toEqual([])
})

test('quiet hours and sound settings persist and are accessible', async ({ page }) => {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Notifications', exact: true }).click()
  await page.getByRole('switch', { name: 'Quiet hours' }).click()
  await page.getByLabel('From', { exact: true }).fill('23:00')
  await page.getByLabel('From', { exact: true }).blur()
  await page.getByRole('radio', { name: 'All' }).last().click()
  expect(await violations(page)).toEqual([])
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Notifications', exact: true }).click()
  await expect(page.getByRole('switch', { name: 'Quiet hours' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await expect(page.getByLabel('From', { exact: true })).toHaveValue('23:00')
  await expect(
    page
      .getByRole('radiogroup', { name: 'Notification sound' })
      .getByRole('radio', { name: 'All' }),
  ).toHaveAttribute('aria-checked', 'true')
})

test('shows the last workspace read-only when the server is unreachable', async ({ page }) => {
  await expect(page.locator('.message-text').first()).toBeVisible()
  await page.waitForTimeout(3500) // the snapshot is saved a few seconds after changes
  await page.route('**/api/**', (route) => route.abort('connectionrefused'))
  await page.reload()
  await expect(page.getByText(/Offline: showing your workspace as of/)).toBeVisible()
  await expect(page.locator('.conversation-link').filter({ hasText: 'development' })).toBeVisible()
  await expect(page.locator('.message-text').first()).toBeVisible()
  // Signing out forgets this device's copy.
  await page.unroute('**/api/**')
  await page.reload()
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('button', { name: 'Create an account' })).toBeVisible()
  expect(
    await page.evaluate(() =>
      Object.keys(localStorage).filter((k) => /snapshot|lastAccount/.test(k)),
    ),
  ).toEqual([])
})

test('server link previews render as cards once a channel opts in', async ({ page }) => {
  await link(page, 'general').click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Channel settings…' }).click()
  await page.getByRole('switch', { name: 'Server link previews' }).click()
  await expect(page.getByRole('switch', { name: 'Server link previews' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
  await page.keyboard.press('Escape')
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('read https://example.com/guides/irc')
  await input.press('Enter')
  const card = page.locator('.link-card').first()
  await expect(card).toContainText('example.com · irc')
  await expect(card).toHaveAttribute('href', 'https://example.com/guides/irc')
  expect(await violations(page)).toEqual([])
})

test('shows who is typing until their message arrives', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('anyone there?')
  await input.press('Enter')
  await expect(page.locator('.typing-line')).toHaveText('echo is typing…')
  await expect(page.locator('.message-text').filter({ hasText: /^anyone there\?$/ })).toHaveCount(2)
  await expect(page.locator('.typing-line')).toHaveCount(0)
})

test('typing state follows the draft and stops when switching away', async ({ page }) => {
  const frames: { conversationId: string; state: string }[] = []
  page.on('websocket', (ws) =>
    ws.on('framesent', ({ payload }) => {
      const frame = JSON.parse(String(payload))
      if (frame.type === 'typing.set') frames.push(frame)
    }),
  )
  await page.reload()
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.pressSequentially('hel')
  await expect
    .poll(() => frames)
    .toEqual([{ v: 1, type: 'typing.set', conversationId: 'general', state: 'active' }])
  await page.locator('.conversation-link').filter({ hasText: 'design' }).click()
  await expect.poll(() => frames.at(-1)).toMatchObject({ conversationId: 'general', state: 'done' })
})
