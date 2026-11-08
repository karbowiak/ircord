import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

const content = (page: Page) => page.locator('.settings-content')

async function openNotifications(page: Page) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.locator('.settings-nav').getByRole('button', { name: 'Notifications', exact: true }).click()
}

test('device timing defaults, explains the last-device window, saves and resets', async ({
  page,
}) => {
  await signIn(page)
  await openNotifications(page)
  const last = content(page).getByRole('spinbutton', { name: 'Notify the last-used device after' })
  const all = content(page).getByRole('spinbutton', { name: 'Notify all devices after' })
  const summary = content(page).getByTestId('routing-summary')
  await expect(last).toHaveValue('30')
  await expect(all).toHaveValue('5')
  await expect(summary).toContainText('for 4 minutes 30 seconds, then every device is')
  await expect(content(page).getByRole('button', { name: 'Reset to defaults' })).toBeDisabled()

  let saved = page.waitForResponse(
    (r) => r.url().includes('/notifications/settings') && r.request().method() === 'PUT' && r.ok(),
  )
  await last.fill('15')
  await last.press('Tab')
  await all.fill('2')
  await all.press('Tab')
  await saved
  await expect(summary).toContainText('for 1 minute 45 seconds')

  // A last-device delay past the all-devices one raises it along: never all before the last.
  await last.fill('300')
  await last.press('Tab')
  await expect(all).toHaveValue('5')
  await expect(summary).toContainText('Every device is notified at once.')

  saved = page.waitForResponse(
    (r) => r.url().includes('/notifications/settings') && r.request().method() === 'PUT' && r.ok(),
  )
  await content(page).getByRole('switch', { name: 'Set me away automatically' }).click()
  await saved
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await openNotifications(page)
  // 300 seconds reads as 5 minutes.
  await expect(content(page).getByRole('spinbutton', { name: 'Notify the last-used device after' })).toHaveValue('5')
  await expect(content(page).getByRole('combobox', { name: 'Notify the last-used device after unit' })).toContainText('minutes')
  await expect(content(page).getByRole('switch', { name: 'Set me away automatically' })).toHaveAttribute('aria-checked', 'false')

  await content(page).getByRole('button', { name: 'Reset to defaults' }).click()
  await expect(content(page).getByRole('spinbutton', { name: 'Notify the last-used device after' })).toHaveValue('30')
  await expect(content(page).getByTestId('routing-summary')).toContainText('4 minutes 30 seconds')
})

test('with server routing, the tab reports activity and alerts only for routed events', async ({
  page,
}) => {
  // Record page notifications instead of showing them; this device notifies “while open”.
  await page.addInitScript(() => {
    const shown: { title: string; body?: string; tag?: string }[] = []
    ;(window as unknown as { shown: typeof shown }).shown = shown
    class FakeNotification {
      static permission = 'granted'
      static requestPermission = async () => 'granted'
      onclick: (() => void) | null = null
      constructor(title: string, options: { body?: string; tag?: string } = {}) {
        shown.push({ title, body: options.body, tag: options.tag })
      }
      close() {}
    }
    ;(window as unknown as { Notification: unknown }).Notification = FakeNotification
    localStorage.setItem('ircord.device.notifications', JSON.stringify('open'))
  })
  const reports: { tab: string; active: boolean; formFactor?: string; canAlert?: boolean }[] = []
  await page.route('**/api/v1/activity', async (route) => {
    reports.push(route.request().postDataJSON())
    await route.fulfill({ status: 204 })
  })
  let resumeTab = ''
  let send: ((value: string) => void) | undefined
  await page.routeWebSocket('**/api/v1/events*', (socket) => {
    send = (value) => socket.send(value)
    const server = socket.connectToServer()
    server.onMessage((raw) => {
      const event = JSON.parse(String(raw))
      // The demo backend does not route; this server does.
      if (event.type === 'session.ready') event.routing = true
      socket.send(JSON.stringify(event))
    })
    socket.onMessage((raw) => {
      const command = JSON.parse(String(raw))
      if (command.type === 'session.resume') resumeTab = command.tab
      server.send(String(raw))
    })
  })
  await signIn(page)

  await expect.poll(() => reports.some((r) => r.active)).toBe(true)
  const tab = reports.find((r) => r.active)!.tab
  expect(tab).toBe(resumeTab)
  expect(tab).toMatch(/^[\w-]{8,64}$/)
  expect(reports.find((r) => r.active)!.formFactor).toBe('desktop')
  // Only the activity flag, the tab, the form factor and whether page notifications show are sent.
  expect(reports.find((r) => r.active)!.canAlert).toBe(true)
  expect(Object.keys(reports.find((r) => r.active)!).sort()).toEqual(['active', 'canAlert', 'formFactor', 'tab'])

  // Hiding the page ends the lease.
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect.poll(() => reports.at(-1)?.active).toBe(false)

  // A routed reminder shows one notification that leads with the unread count.
  send!(
    JSON.stringify({
      v: 1,
      type: 'notification.alert',
      data: {
        conversationId: 'general',
        messageId: 'm9',
        title: 'mika in #general',
        body: 'still there?',
        kind: 'mention',
        badge: 3,
        unread: 3,
        conversations: 2,
        reminder: true,
      },
    }),
  )
  await expect
    .poll(() => page.evaluate(() => (window as unknown as { shown: { title: string }[] }).shown))
    .toEqual([
      { title: '3 unread notifications in 2 conversations', body: 'mika in #general: still there?', tag: 'general' },
    ])

  // Coming back renews activity straight away.
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'visible' })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect.poll(() => reports.at(-1)?.active).toBe(true)

  // Turning page notifications off tells the server this open page can no longer alert.
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.locator('.settings-nav').getByRole('button', { name: 'Notifications', exact: true }).click()
  await content(page).getByRole('radiogroup', { name: 'Notifications on this device' }).getByRole('radio', { name: 'Off' }).click()
  await expect.poll(() => reports.at(-1)?.canAlert).toBe(false)
})
