import { expect, test, type Page } from '@playwright/test'
import { signIn, testPassword } from './helpers'

const firefox = 'Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0'
const iphone =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1'

async function openDevices(page: Page) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  const list = page.getByRole('list', { name: 'Signed-in devices' })
  await expect(list).toBeVisible()
  return list
}
async function signInAs(page: Page, username: string) {
  await page.goto('/')
  await page.getByLabel('Username', { exact: true }).fill(username)
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
}

test('lists signed-in devices and signs a remote one out after a confirmation', async ({
  browser,
}) => {
  const laptopContext = await browser.newContext({ userAgent: firefox })
  const laptop = await laptopContext.newPage()
  const username = await signIn(laptop)
  const phoneContext = await browser.newContext({ userAgent: iphone })
  const phone = await phoneContext.newPage()
  await signInAs(phone, username)
  // A second tab of the phone's browser is the same device, not a new one.
  const phoneTab = await phoneContext.newPage()
  await phoneTab.goto('/')
  await expect(phoneTab.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()

  const list = await openDevices(laptop)
  await expect(list.getByRole('listitem')).toHaveCount(2)
  const mine = list.getByRole('listitem', { name: 'Firefox on Linux, this device' })
  await expect(mine).toContainText('This device')
  await expect(mine).toContainText('Active now')
  const target = list.getByRole('listitem', { name: 'Safari on iOS', exact: true })
  await expect(target).toContainText(/Last seen just now|Connected now/)
  await expect(target).toContainText('No push')
  // Exact sign-in time is available on hover.
  await expect(target.getByText(/Signed in just now/)).toHaveAttribute('title', /\d/)

  // Cancelling changes nothing.
  await target.getByRole('button', { name: 'Sign out Safari on iOS' }).click()
  const dialog = laptop.getByRole('dialog', { name: 'Sign out Safari on iOS?' })
  await expect(dialog).toContainText('Every IRCord tab on that device signs out')
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(target).toBeVisible()

  // A failed request does not report the device as signed out.
  await laptop.route('**/api/v1/auth/devices/*', (route) =>
    route.fulfill({ status: 503, contentType: 'application/json', body: '{"title":"Service Unavailable","detail":"Account storage is unavailable."}' }),
  )
  await target.getByRole('button', { name: 'Sign out Safari on iOS' }).click()
  await dialog.getByRole('button', { name: 'Sign out device' }).click()
  await expect(target.getByRole('alert')).toContainText('Couldn’t sign out Safari on iOS')
  await expect(target).toBeVisible()
  await laptop.unroute('**/api/v1/auth/devices/*')

  await target.getByRole('button', { name: 'Sign out Safari on iOS' }).click()
  const revoke = laptop.waitForResponse('**/api/v1/auth/devices/*')
  await dialog.getByRole('button', { name: 'Sign out device' }).click()
  expect((await revoke).status()).toBe(204)
  await expect(laptop.getByRole('status').filter({ hasText: 'Signed out Safari on iOS.' })).toBeVisible()
  await expect(list.getByRole('listitem')).toHaveCount(1)

  // Both of the phone's tabs are signed out; the laptop is not.
  for (const tab of [phone, phoneTab]) {
    await tab.reload()
    await expect(tab.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  }
  await laptop.reload()
  await expect(laptop.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()

  // Signing in again is a new device, listed afresh.
  await signInAs(phone, username)
  const again = await openDevices(laptop)
  await expect(again.getByRole('listitem')).toHaveCount(2)
  await laptopContext.close()
  await phoneContext.close()
})

test('signing out this device explains itself and returns to the sign-in screen', async ({
  page,
}) => {
  await signIn(page)
  const list = await openDevices(page)
  await list.getByRole('button', { name: 'Sign out this device' }).click()
  const dialog = page.getByRole('dialog', { name: 'Sign out this device?' })
  await expect(dialog).toContainText('This is the device you’re using.')
  await dialog.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  expect(
    await page.evaluate(() => Object.keys(localStorage).filter((k) => /snapshot|lastAccount/.test(k))),
  ).toEqual([])
})

test('Devices & sessions fits a phone screen', async ({ browser }) => {
  const context = await browser.newContext({ userAgent: firefox })
  const page = await context.newPage()
  const username = await signIn(page)
  const phone = await browser.newContext({ userAgent: iphone })
  await signInAs(await phone.newPage(), username)
  const list = await openDevices(page)
  await page.setViewportSize({ width: 390, height: 844 })
  const target = list.getByRole('listitem', { name: 'Safari on iOS', exact: true })
  await target.scrollIntoViewIfNeeded()
  await expect(target.getByRole('button', { name: 'Sign out Safari on iOS' })).toBeInViewport()
  const content = page.locator('.settings-content')
  expect(await content.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  if (process.env.DEVICES_SCREENSHOT)
    await page.screenshot({ path: process.env.DEVICES_SCREENSHOT })
  await context.close()
  await phone.close()
})
