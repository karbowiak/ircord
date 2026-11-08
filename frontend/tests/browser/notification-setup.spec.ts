import { expect, test, type BrowserContext, type Page } from '@playwright/test'
import { signIn } from './helpers'

/*
 * First-use notification setup and the device summary (#189). Notification permission and the
 * browser's push service are faked so every state is reachable; the demo server stores the
 * registration per signed-in device like the backend. Real phone delivery is not covered here.
 */

const iphone =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1'
const android =
  'Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Mobile Safari/537.36'
const phone = { viewport: { width: 412, height: 860 }, isMobile: true, hasTouch: true }

interface Fake {
  /** The permission the page starts with, unless a test changed it (kept across reloads). */
  permission?: NotificationPermission
  /** What the native permission prompt answers. */
  answer?: NotificationPermission
  /** iOS outside the Home Screen: no Notification API at all. */
  noNotifications?: boolean
}
/** Fakes Notification and the push service; window.__fake records native prompts. */
async function fakeBrowser(context: BrowserContext, fake: Fake = {}) {
  await context.addInitScript((fake: Fake) => {
    const state = window as unknown as { __fake: { prompts: number } }
    state.__fake = { prompts: 0 }
    const read = (key: string) => sessionStorage.getItem(`fake.${key}`)
    const write = (key: string, value: string | null) =>
      value === null
        ? sessionStorage.removeItem(`fake.${key}`)
        : sessionStorage.setItem(`fake.${key}`, value)
    if (fake.noNotifications) {
      delete (window as { Notification?: unknown }).Notification
    } else {
      class FakeNotification {
        static get permission() {
          return read('permission') ?? fake.permission ?? 'default'
        }
        static async requestPermission() {
          state.__fake.prompts++
          const answer = fake.answer ?? 'granted'
          write('permission', answer)
          return answer
        }
        onclick: (() => void) | null = null
        close() {}
      }
      ;(window as unknown as { Notification: unknown }).Notification = FakeNotification
    }
    const subscription = () => {
      const endpoint = read('endpoint')
      if (!endpoint) return null
      return {
        endpoint,
        toJSON: () => ({
          endpoint,
          expirationTime: null,
          keys: { p256dh: 'B'.repeat(87), auth: 'A'.repeat(22) },
        }),
        unsubscribe: async () => (write('endpoint', null), true),
      }
    }
    const registration = {
      pushManager: {
        getSubscription: async () => subscription(),
        subscribe: async () => {
          write(
            'endpoint',
            read('endpoint') ??
              `https://fcm.googleapis.com/fcm/send/fake-${Math.random().toString(36).slice(2)}`,
          )
          return subscription()
        },
      },
    }
    Object.defineProperty(navigator.serviceWorker, 'ready', {
      get: () => Promise.resolve(registration),
    })
  }, fake)
}
/** Pretends the user changed the permission in browser or OS settings, then came back. */
async function settingsChanged(page: Page, permission: NotificationPermission) {
  await page.evaluate((p) => {
    sessionStorage.setItem('fake.permission', p)
    document.dispatchEvent(new Event('visibilitychange'))
  }, permission)
}
const prompts = (page: Page) =>
  page.evaluate(() => (window as unknown as { __fake: { prompts: number } }).__fake.prompts)
const banner = (page: Page) =>
  page.getByRole('region', { name: /^(Get notified|Want notifications|Finish setting up)/ })

async function openNotifications(page: Page) {
  // On a phone the settings button sits in the navigation drawer.
  const open = page.getByRole('button', { name: 'User settings', exact: true })
  if (!(await open.isVisible()))
    await page.getByRole('button', { name: 'Open conversations' }).click()
  await open.click()
  await page
    .locator('.settings-nav')
    .getByRole('button', { name: 'Notifications', exact: true })
    .click()
  return page.locator('[data-setting="notifications-status"]')
}
async function workspace(page: Page) {
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
}

test('a phone is offered setup once; Enable asks for permission only on the click', async ({
  browser,
}) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context)
  const page = await context.newPage()
  const posted: string[] = []
  page.on('request', (r) => {
    if (r.url().includes('/api/v1/push/subscriptions') && r.method() === 'POST')
      posted.push(r.url())
  })
  await signIn(page)
  const offer = banner(page)
  await expect(offer).toContainText('Get notified on this device?')
  // Nothing native happened on its own.
  expect(await prompts(page)).toBe(0)
  expect(posted).toHaveLength(0)

  await offer.getByRole('button', { name: 'Enable' }).click()
  await expect(offer).toBeHidden()
  expect(await prompts(page)).toBe(1)
  await expect.poll(() => posted.length).toBe(1)

  // Configured: no offer after a reload, and the summary says push is registered.
  await page.reload()
  await workspace(page)
  await page.waitForTimeout(2500)
  await expect(banner(page)).toBeHidden()
  const status = await openNotifications(page)
  await expect(status).toContainText('On, even when IRCord is closed')
  await expect(status).toContainText('Browser permission')
  await expect(status).toContainText('Allowed')
  await expect(status).toContainText('Registered')

  // The test reaches this device only, and only when asked.
  let tests = 0
  await page.route('**/api/v1/push/test', (route) => (tests++, route.fulfill({ status: 204 })))
  await expect(page.locator('[data-setting="notifications-test"]')).toContainText(
    'this device only',
  )
  expect(tests).toBe(0)
  await page
    .locator('[data-setting="notifications-test"]')
    .getByRole('button', { name: 'Send test' })
    .click()
  await expect(page.getByRole('status').filter({ hasText: 'arrive on this device' })).toBeVisible()
  expect(tests).toBe(1)
  await context.close()
})

test('Not now is remembered across reloads and Settings still offers setup', async ({
  browser,
}) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context)
  const page = await context.newPage()
  await signIn(page)
  await banner(page).getByRole('button', { name: 'Not now' }).click()
  await expect(banner(page)).toBeHidden()
  expect(await prompts(page)).toBe(0)
  await page.reload()
  await workspace(page)
  await page.waitForTimeout(2500)
  await expect(banner(page)).toBeHidden()

  const status = await openNotifications(page)
  await expect(status).toContainText('Off on this device')
  await expect(status).toContainText('Not asked yet')
  await status.getByRole('button', { name: 'Enable notifications' }).click()
  await expect(status).toContainText('On, even when IRCord is closed')
  expect(await prompts(page)).toBe(1)
  await context.close()
})

test('the offer waits for open dialogs instead of stacking over them', async ({ browser }) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context)
  const page = await context.newPage()
  await signIn(page)
  await expect(banner(page)).toBeVisible()
  await openNotifications(page)
  await expect(banner(page)).toBeHidden()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toBeHidden()
  // The navigation drawer that led to Settings is still open: it waits for that too.
  await expect(banner(page)).toBeHidden()
  await page.keyboard.press('Escape')
  await expect(banner(page)).toBeVisible()
  await context.close()
})

test('a blocked permission is never prompted; Settings explains and rereads it on return', async ({
  browser,
}) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context, { permission: 'denied' })
  const page = await context.newPage()
  await signIn(page)
  await page.waitForTimeout(2500)
  await expect(banner(page)).toBeHidden()
  const status = await openNotifications(page)
  await expect(status).toContainText('Blocked by your browser')
  await expect(status).toContainText('system settings')
  await expect(page.getByRole('radio', { name: 'Always' })).toBeDisabled()

  // Allowed in the browser's settings: the summary follows without a reload.
  await settingsChanged(page, 'granted')
  await expect(status).toContainText('Off on this device')
  await expect(status).toContainText('Allowed')
  expect(await prompts(page)).toBe(0)
  await context.close()
})

test('a lost push registration is offered as a repair, without a new permission prompt', async ({
  browser,
}) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context)
  const page = await context.newPage()
  await signIn(page)
  await banner(page).getByRole('button', { name: 'Enable' }).click()
  await expect(banner(page)).toBeHidden()
  expect(await prompts(page)).toBe(1)

  // The browser and the server both lost the subscription, and re-registering on load fails.
  await page.evaluate(async () => {
    const endpoint = sessionStorage.getItem('fake.endpoint')
    await fetch('/api/v1/push/subscriptions', {
      method: 'DELETE',
      credentials: 'include',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ endpoint }),
    })
    sessionStorage.removeItem('fake.endpoint')
  })
  await page.route('**/api/v1/push/subscriptions', (route) =>
    route.request().method() === 'POST'
      ? route.fulfill({ status: 503, json: { title: 'Service Unavailable', detail: 'Try again.' } })
      : route.continue(),
  )
  await page.reload()
  await workspace(page)
  const offer = banner(page)
  await expect(offer).toContainText('Finish setting up notifications')
  await page.unroute('**/api/v1/push/subscriptions')
  await offer.getByRole('button', { name: 'Repair' }).click()
  await expect(offer).toBeHidden()
  expect(await prompts(page), 'permission was already granted').toBe(0)
  const status = await openNotifications(page)
  await expect(status).toContainText('On, even when IRCord is closed')
  await context.close()
})

test('a server without Web Push never offers push or a false “enabled”', async ({ browser }) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context)
  const page = await context.newPage()
  await page.route('**/api/v1/push/key', (route) =>
    route.fulfill({ json: { publicKey: '', enabled: false } }),
  )
  await signIn(page)
  await page.waitForTimeout(2500)
  await expect(banner(page)).toBeHidden()
  const status = await openNotifications(page)
  await expect(status).toContainText('Unavailable on this server')
  await expect(page.getByRole('radio', { name: 'Always' })).toBeDisabled()
  // The one-step setup falls back to while-open.
  await status.getByRole('button', { name: 'Enable notifications' }).click()
  await expect(status).toContainText('On while IRCord is open')
  await context.close()
})

test('an iPhone browser tab gets Home Screen help and no Install button or native prompt', async ({
  browser,
}) => {
  const context = await browser.newContext({ ...phone, userAgent: iphone })
  await fakeBrowser(context, { noNotifications: true })
  const page = await context.newPage()
  await signIn(page)
  const offer = banner(page)
  await expect(offer).toContainText('Add to Home Screen')
  await expect(offer.getByRole('button', { name: 'Install' })).toHaveCount(0)
  await expect(offer.getByRole('button', { name: 'Enable' })).toHaveCount(0)
  await offer.getByRole('button', { name: 'Not now' }).click()
  const status = await openNotifications(page)
  await expect(status).toContainText('Needs the Home Screen app')
  await expect(page.locator('[data-setting="install-app"]')).toContainText('doesn’t carry over')
  await context.close()
})

test('a desktop tab is not prompted', async ({ page, context }) => {
  await fakeBrowser(context)
  await signIn(page)
  await page.waitForTimeout(2500)
  await expect(banner(page)).toBeHidden()
})

test('another account signing in on the same device does not inherit push', async ({ browser }) => {
  const context = await browser.newContext({ ...phone, userAgent: android })
  await fakeBrowser(context)
  const page = await context.newPage()
  await signIn(page)
  await banner(page).getByRole('button', { name: 'Enable' }).click()
  await expect(banner(page)).toBeHidden()
  // Sign out, then a second account signs in here.
  await page.evaluate(() =>
    fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' }),
  )
  await page.reload()
  await page.getByRole('button', { name: 'Create an account' }).waitFor()
  const posted: string[] = []
  page.on('request', (r) => {
    if (r.url().includes('/api/v1/push/subscriptions') && r.method() === 'POST')
      posted.push(r.url())
  })
  await signIn(page)
  // Offered as a new account on this device, and nothing registered without asking.
  await expect(banner(page)).toContainText('Get notified on this device?')
  expect(posted).toHaveLength(0)
  await banner(page).getByRole('button', { name: 'Not now' }).click()
  const status = await openNotifications(page)
  await expect(status).toContainText('Off on this device')
  expect(posted).toHaveLength(0)
  await context.close()
})
