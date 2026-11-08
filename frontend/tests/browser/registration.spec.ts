import { test, expect } from '@playwright/test'
import { signIn, testPassword, uniqueUsername } from './helpers'

test('signup checks the username against the network and gates on a shared key', async ({
  page,
}) => {
  await page.route('**/api/v1/auth/registration', (route) =>
    route.fulfill({ json: { registrationEnabled: true, registrationKeyRequired: true } }),
  )
  await page.route('**/api/v1/auth/signup', (route) =>
    route.request().postDataJSON()?.registrationKey === 'browser-fixture-key'
      ? route.continue()
      : route.fulfill({
          status: 403,
          json: { detail: 'Registration key is missing or incorrect.' },
        }),
  )
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  const name = page.getByLabel('Username', { exact: true })
  await expect(name).toBeFocused()
  const hint = page.locator('#signup-username-hint')
  const create = page.getByRole('button', { name: 'Create account', exact: true })

  // Names on the network, and names that aren't valid nicks, are refused while typing.
  await name.fill('mika')
  await expect(hint).toHaveText('mika is already taken.')
  await expect(name).toHaveAttribute('aria-invalid', 'true')
  await name.fill('two words')
  await expect(hint).toContainText('can’t contain spaces')

  const username = uniqueUsername()
  await name.fill(username)
  await expect(hint).toContainText(`${username} is available`)
  await page.getByLabel('Password', { exact: true }).fill('short')
  await expect(create).toBeDisabled()
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  const key = page.getByLabel('Registration key', { exact: true })
  await expect(key).toHaveAttribute('type', 'text')
  await expect(key).toHaveAttribute('autocomplete', 'off')
  await expect(page.getByLabel('Password', { exact: true })).toHaveAttribute('type', 'password')
  await expect(create).toBeDisabled()
  await key.fill('wrong')
  await create.click()
  await expect(page.getByRole('alert')).toContainText('missing or incorrect')
  await page.locator('.login-card').screenshot({ path: '.impeccable/review/signup-desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.locator('.login-card').screenshot({ path: '.impeccable/review/signup-mobile.png' })
  await key.fill('browser-fixture-key')
  await create.click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})

test('closed registration and failed policy lookup still allow signing in', async ({ page }) => {
  await page.route('**/api/v1/auth/registration', (route) =>
    route.fulfill({ json: { registrationEnabled: false, registrationKeyRequired: true } }),
  )
  await page.goto('/')
  await expect(page.getByText('New registrations are currently closed.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create an account' })).toHaveCount(0)
  await expect(page.getByLabel('Username', { exact: true })).toBeVisible()
  await page.unroute('**/api/v1/auth/registration')
  await page.route('**/api/v1/auth/registration', (route) =>
    route.fulfill({ status: 503, json: { detail: 'Unavailable' } }),
  )
  await page.reload()
  await expect(page.locator('.form-error')).toContainText('could not be checked')
  await expect(page.getByRole('button', { name: 'Create an account' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
})

test('changes the password', async ({ page }) => {
  const username = await signIn(page)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  const change = page.getByRole('button', { name: 'Change password' })
  await page.getByLabel('Current password').fill('wrong-password')
  await page.getByLabel('New password').fill('a-brand-new-password')
  await change.click()
  await expect(page.getByRole('alert')).toHaveText('Your current password is wrong.')
  await page.getByLabel('Current password').fill(testPassword)
  await change.click()
  await expect(page.getByRole('status').filter({ hasText: 'Password changed.' })).toBeVisible()

  await page.getByRole('button', { name: 'Sign out' }).click()
  await page.getByLabel('Username', { exact: true }).fill(username)
  await page.getByLabel('Password', { exact: true }).fill('a-brand-new-password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})

test('signup remains available after a transient username lookup failure', async ({ page }) => {
  await page.route('**/api/v1/auth/username?*', route => route.fulfill({ status: 503, json: { detail: 'Unavailable' } }))
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await page.getByLabel('Username', { exact: true }).fill(uniqueUsername())
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  await expect(page.getByRole('button', { name: 'Create account', exact: true })).toBeEnabled()
  await page.getByRole('button', { name: 'Create account', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})
