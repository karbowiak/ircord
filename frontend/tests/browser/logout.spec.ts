import { expect, test, type Page } from '@playwright/test'
import { signIn, testPassword } from './helpers'

const logOutEntry = (page: Page) =>
  page.getByRole('navigation', { name: 'Settings' }).getByRole('button', { name: 'Log out' })
const confirmation = (page: Page) => page.getByRole('dialog', { name: 'Log out of IRCord?' })
/** This device's copy of the account: the offline snapshot and the last-account record. */
const deviceKeys = (page: Page) =>
  page.evaluate(() => Object.keys(localStorage).filter((k) => /snapshot|lastAccount/.test(k)))

test('the settings menu logs out of this session after a confirmation', async ({
  page,
  browser,
}) => {
  const username = await signIn(page)
  // A second device on the same account must stay signed in.
  const other = await browser.newContext()
  const second = await other.newPage()
  await second.goto('/')
  await second.getByLabel('Username', { exact: true }).fill(username)
  await second.getByLabel('Password', { exact: true }).fill(testPassword)
  await second.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(second.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()

  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  const entry = logOutEntry(page)
  // Destructive styling: the theme's live --danger colour, whatever the palette.
  const danger = await page.evaluate(() => {
    const probe = document.createElement('span')
    probe.style.color = 'var(--danger)'
    document.body.append(probe)
    const color = getComputedStyle(probe).color
    probe.remove()
    return color
  })
  await expect(entry).toHaveCSS('color', danger)

  // Keyboard: focus the entry and open the confirmation; Escape closes only the confirmation.
  await entry.focus()
  await page.keyboard.press('Enter')
  await expect(confirmation(page)).toBeVisible()
  await expect(confirmation(page).getByRole('button', { name: 'Cancel' })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(confirmation(page)).toHaveCount(0)
  await expect(page.getByRole('dialog', { name: 'Settings' })).toBeVisible()

  // Cancel changes nothing either.
  await entry.click()
  await confirmation(page).getByRole('button', { name: 'Cancel' }).click()
  await expect(confirmation(page)).toHaveCount(0)
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()

  // Confirming signs this session out and returns to the login screen.
  // The snapshot is saved a few seconds after the workspace settles; logging out must remove it.
  await expect.poll(() => deviceKeys(page), { timeout: 10_000 }).toHaveLength(2)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await logOutEntry(page).click()
  const logout = page.waitForResponse('**/api/v1/auth/logout')
  await confirmation(page).getByRole('button', { name: 'Log out' }).click()
  expect((await logout).status()).toBe(204)
  await expect(page.getByLabel('Username', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
  expect(await deviceKeys(page)).toEqual([])
  await page.reload()
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()

  // The other session is untouched.
  await second.reload()
  await expect(second.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await other.close()
})

test('the Account tab’s Log out asks the same question first', async ({ page }) => {
  await signIn(page)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  const button = page.locator('.settings-content').getByRole('button', { name: 'Log out' })
  await button.click()
  await confirmation(page).getByRole('button', { name: 'Cancel' }).click()
  await expect(confirmation(page)).toHaveCount(0)
  await expect(page.locator('.account-summary')).toBeVisible()
  await button.click()
  await confirmation(page).getByRole('button', { name: 'Log out' }).click()
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
})
