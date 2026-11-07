import { test, expect } from '@playwright/test'

test('gated registration checks the shared key and keeps personal-key login separate', async ({
  page,
}) => {
  await page.route('**/api/v1/auth/registration', (route) =>
    route.fulfill({ json: { registrationEnabled: true, registrationKeyRequired: true } }),
  )
  await page.route('**/api/v1/auth/register', (route) => {
    const body = route.request().postDataJSON()
    return body?.registrationKey === 'browser-fixture-key'
      ? route.continue()
      : route.fulfill({
          status: 403,
          json: { detail: 'Registration key is missing or incorrect.' },
        })
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account', exact: true }).click()
  const input = page.getByLabel('Registration key', { exact: true })
  await expect(input).toBeFocused()
  await expect(input).toHaveAttribute('type', 'password')
  await expect(page.getByRole('button', { name: 'Create an account', exact: true })).toBeDisabled()
  await input.fill('wrong')
  await page.getByRole('button', { name: 'Create an account', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('missing or incorrect')
  await expect(page.getByRole('heading', { name: 'Save your account key' })).toHaveCount(0)
  await page
    .locator('.login-card')
    .screenshot({ path: '.impeccable/review/registration-key-desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await page
    .locator('.login-card')
    .screenshot({ path: '.impeccable/review/registration-key-mobile.png' })
  await input.fill('browser-fixture-key')
  await page.getByRole('button', { name: 'Create an account', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Save your account key' })).toBeVisible()
  const personal = (await page.getByLabel('Your account key').innerText()).trim()
  expect(personal).toMatch(/^ircord-/)
  await page.getByLabel('I’ve saved my key somewhere safe').check()
  await page.getByRole('button', { name: 'Continue to IRCord' }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await page.request.post('/api/v1/auth/logout')
  await page.reload()
  await page.getByRole('button', { name: 'I have a key', exact: true }).click()
  await page.getByLabel('Account key', { exact: true }).fill(personal)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})

test('closed registration and failed policy lookup preserve the existing-login path', async ({
  page,
}) => {
  await page.route('**/api/v1/auth/registration', (route) =>
    route.fulfill({ json: { registrationEnabled: false, registrationKeyRequired: true } }),
  )
  await page.goto('/')
  await expect(page.getByText('New registrations are currently closed.')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create an account', exact: true })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'I have a key', exact: true })).toBeEnabled()
  await page.unroute('**/api/v1/auth/registration')
  await page.route('**/api/v1/auth/registration', (route) =>
    route.fulfill({ status: 503, json: { detail: 'Unavailable' } }),
  )
  await page.reload()
  await expect(page.getByRole('alert')).toContainText('could not be checked')
  await expect(page.getByRole('button', { name: 'Create an account', exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'I have a key', exact: true }).click()
  await expect(page.getByLabel('Account key', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible()
})
