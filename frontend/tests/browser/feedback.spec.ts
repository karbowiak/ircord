import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => signIn(page))

test('address verification blocks invalid servers, recovers after editing, and can be canceled', async ({
  page,
}) => {
  let created = 0
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().endsWith('/api/v1/networks')) created++
  })
  await page.getByRole('button', { name: 'Add a network', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Add a network', exact: true })
  await dialog.getByLabel('Name', { exact: true }).fill('Address check')
  await dialog.getByLabel('Hostname', { exact: true }).fill('missing.invalid')
  await expect(dialog.getByRole('status')).toContainText('could not be found')
  await expect(dialog.getByRole('button', { name: 'Add network', exact: true })).toBeDisabled()
  expect(created).toBe(0)
  await dialog.getByRole('button', { name: 'Retry verification', exact: true }).click()
  await expect(dialog.getByRole('status')).toContainText('could not be found')
  await dialog.getByLabel('Hostname', { exact: true }).fill('irc.example.org')
  await expect(dialog.getByRole('status')).toContainText('Server address verified')
  await expect(dialog.getByRole('button', { name: 'Add network', exact: true })).toBeEnabled()
  await page.route('**/api/v1/networks/check', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 1500))
    await route
      .fulfill({ json: { reachable: true, detail: 'Server address verified.' } })
      .catch(() => {})
  })
  await dialog.getByLabel('Hostname', { exact: true }).fill('slow.example.org')
  await expect(dialog.getByRole('button', { name: 'Verifying…', exact: true })).toBeDisabled()
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(dialog).toBeHidden()
  expect(created).toBe(0)
})

test('empty networks open their own Server view instead of another network conversation', async ({
  page,
}) => {
  const response = await page.request.post('/api/v1/networks', {
    data: {
      name: 'Empty network',
      host: 'irc.example.org',
      port: 6697,
      tls: true,
      nick: 'empty',
      auth: { method: 'none' },
      autoJoin: [],
    },
  })
  expect(response.ok()).toBe(true)
  await page.getByRole('button', { name: 'Empty network', exact: true }).click()
  await expect(
    page.getByRole('log', { name: 'Empty network server messages', exact: true }),
  ).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Message #general', exact: true })).toHaveCount(0)
  await page.reload()
  await expect(
    page.getByRole('log', { name: 'Empty network server messages', exact: true }),
  ).toBeVisible()
  await page.getByRole('button', { name: 'Empty network', exact: true }).click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Remove network…', exact: true }).click()
  const confirm = page.getByRole('dialog', { name: 'Remove Empty network?', exact: true })
  await confirm.getByRole('textbox').fill('Empty network')
  await confirm.getByRole('button', { name: 'Remove', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Empty network', exact: true })).toHaveCount(0)
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
})

test('About you preserves multiline text across settings and reload, and reports failed saves', async ({
  page,
}) => {
  const bio = 'First line\nSecond line — hello 👋'
  const openProfile = async () => {
    await page.getByRole('button', { name: 'User settings', exact: true }).click()
    await page.getByRole('button', { name: 'Profile', exact: true }).click()
  }
  await openProfile()
  await page.getByLabel('About you', { exact: true }).fill(bio)
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog', exact: true }).click()
  await openProfile()
  await expect(page.getByLabel('About you', { exact: true })).toHaveValue(bio)
  await page.reload()
  await openProfile()
  await expect(page.getByLabel('About you', { exact: true })).toHaveValue(bio)
  await page.route('**/api/v1/profile', (route) =>
    route.fulfill({ status: 503, json: { detail: 'Profile save unavailable. Try again.' } }),
  )
  await page.getByLabel('About you', { exact: true }).fill('Unsaved draft')
  await expect(page.getByText('Profile saved', { exact: true })).toHaveCount(0)
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByRole('alert')).toContainText('Profile save unavailable')
  await expect(page.getByLabel('About you', { exact: true })).toHaveValue('Unsaved draft')
  await page.unroute('**/api/v1/profile')
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
})

test('links in About you open from the profile and markup stays text', async ({ page }) => {
  const bio = 'Site: https://example.org/about <b>not bold</b>'
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Profile', exact: true }).click()
  await page.getByLabel('About you', { exact: true }).fill(bio)
  await page.getByRole('button', { name: 'Save changes', exact: true }).click()
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Close dialog', exact: true }).click()
  await page.locator('.member-button').filter({ hasText: 'Karbowiak' }).click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Profile' }).click()
  const about = page.locator('.profile-bio')
  const link = about.getByRole('link', { name: 'https://example.org/about', exact: true })
  await expect(link).toHaveAttribute('href', 'https://example.org/about')
  await expect(link).toHaveAttribute('target', '_blank')
  await expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  await expect(about).toContainText('<b>not bold</b>')
  await expect(about.locator('b')).toHaveCount(0)
})
