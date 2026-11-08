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
  await expect(hint).toHaveText('mika is already taken on irc.ircord.com.')
  await expect(name).toHaveAttribute('aria-invalid', 'true')
  await name.fill('two words')
  await expect(hint).toContainText('cannot contain spaces')

  const username = uniqueUsername()
  await name.fill(username)
  await expect(hint).toContainText(`${username} is available`)
  await page.getByLabel('Password', { exact: true }).fill('short')
  await expect(create).toBeDisabled()
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  const key = page.getByLabel('Registration key', { exact: true })
  await expect(key).toHaveAttribute('type', 'password')
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
  // A new account has a username, so it isn't asked to choose one.
  await expect(page.getByRole('dialog', { name: 'Choose a username and password' })).toHaveCount(0)
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
  await page.getByRole('button', { name: /Sign in with it/ }).click()
  await expect(page.getByLabel('Account key', { exact: true })).toBeVisible()
})

test('a key account signs in with its key and chooses a username and password', async ({
  page,
}) => {
  const { key } = await (await page.request.post('/api/v1/auth/register')).json()
  await page.request.post('/api/v1/auth/logout')
  await page.goto('/')
  await page.getByRole('button', { name: /Sign in with it/ }).click()
  await page.getByLabel('Account key', { exact: true }).fill(key)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()

  const prompt = page.getByRole('dialog', { name: 'Choose a username and password' })
  await expect(prompt).toBeVisible()
  // “Not now” lasts for this visit.
  await prompt.getByRole('button', { name: 'Not now' }).click()
  await expect(prompt).toHaveCount(0)
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await expect(prompt).toHaveCount(0)

  // Settings offers it again.
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  await page.getByRole('button', { name: 'Choose', exact: true }).click()
  const username = uniqueUsername()
  await prompt.getByLabel('Username', { exact: true }).fill(username)
  await expect(prompt.locator('#credentials-username-hint')).toContainText('is available')
  await prompt.getByLabel('Password', { exact: true }).fill(testPassword)
  await prompt.getByRole('button', { name: 'Save' }).click()
  await expect(prompt).toHaveCount(0)
  await expect(page.locator('.account-summary strong')).toHaveText(username)
  await expect(page.getByRole('button', { name: 'Create new key' })).toHaveCount(0)

  // From now on the username and password sign in.
  await page.getByRole('button', { name: 'Sign out' }).click()
  await page.getByLabel('Username', { exact: true }).fill(username)
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await expect(prompt).toHaveCount(0)
})

test('changes the password', async ({ page }) => {
  const username = await signIn(page)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Account', exact: true }).click()
  const change = page.getByRole('button', { name: 'Change password' })
  await page.getByLabel('Current password').fill('wrong password')
  await page.getByLabel('New password').fill('a brand new password')
  await change.click()
  await expect(page.getByRole('alert')).toHaveText('Your current password is wrong.')
  await page.getByLabel('Current password').fill(testPassword)
  await change.click()
  await expect(page.getByRole('status').filter({ hasText: 'Password changed.' })).toBeVisible()

  await page.getByRole('button', { name: 'Sign out' }).click()
  await page.getByLabel('Username', { exact: true }).fill(username)
  await page.getByLabel('Password', { exact: true }).fill('a brand new password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})
