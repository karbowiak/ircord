import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => signIn(page))

test('connection settings reject invalid nicknames and save a real name with spaces', async ({
  page,
}) => {
  let connections = 0
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().endsWith('/api/v1/networks')) connections++
  })
  await page.getByRole('button', { name: 'Add a network', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Add a network', exact: true })
  await dialog.getByLabel('Name', { exact: true }).fill('Identity check')
  await dialog.getByLabel('Hostname', { exact: true }).fill('irc.example.org')
  await dialog.getByLabel('Nickname', { exact: true }).fill('Invalid nickname')
  await dialog.getByRole('button', { name: 'Add network', exact: true }).click()
  await expect(dialog.getByLabel('Nickname', { exact: true })).toHaveAttribute(
    'aria-invalid',
    'true',
  )
  await expect(dialog).toContainText('Nicknames cannot contain spaces')
  expect(connections).toBe(0)
  await dialog.getByLabel('Nickname', { exact: true }).fill('valid_nick')
  await dialog.getByLabel('If taken, use', { exact: true }).fill('invalid alternate')
  await dialog.getByRole('button', { name: 'Add network', exact: true }).click()
  expect(connections).toBe(0)
  await dialog.getByLabel('If taken, use', { exact: true }).fill('valid_alt')
  await dialog.getByLabel('Real name', { exact: true }).fill('Display Name With Spaces')
  await dialog.getByRole('button', { name: 'Add network', exact: true }).click()
  await expect(dialog).toBeHidden()
  const state = await (await page.request.get('/api/v1/bootstrap')).json()
  const network = state.networks.find((n: { name: string }) => n.name === 'Identity check')
  expect(network.nick).toBe('valid_nick')
  expect(network.realname).toBe('Display Name With Spaces')
})

test('profile nickname and slash command do not silently shorten a name with spaces', async ({
  page,
}) => {
  const before = await (await page.request.get('/api/v1/bootstrap')).json()
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('/nick Invalid nickname')
  await input.press('Enter')
  await expect(
    page.getByText('Nicknames cannot contain spaces. Your display name (Settings → Profile) can.', {
      exact: true,
    }),
  ).toBeVisible()
  const after = await (await page.request.get('/api/v1/bootstrap')).json()
  expect(after.user.nick).toBe(before.user.nick)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Profile', exact: true }).click()
  const nickname = page.locator('#identity-nick')
  await nickname.fill('Invalid nickname')
  await nickname.locator('..').getByRole('button', { name: 'Change', exact: true }).click()
  expect(await nickname.evaluate((element: HTMLInputElement) => element.validity.valid)).toBe(false)
  await expect(nickname).toHaveAttribute('aria-invalid', 'true')
  const realname = page.locator('#identity-realname')
  await realname.fill('A Real Display Name')
  await realname.locator('..').getByRole('button', { name: 'Change', exact: true }).click()
  await expect
    .poll(async () => {
      const state = await (await page.request.get('/api/v1/bootstrap')).json()
      return state.networks.find((n: { id: string }) => n.id === before.networks[0].id)?.realname
    })
    .toBe('A Real Display Name')
})

test('kick accepts an empty optional reason', async ({ page }) => {
  await page.locator('.member-button').filter({ hasText: 'Noah' }).click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Kick…', exact: true }).click()
  const dialog = page.getByRole('dialog').filter({ hasText: 'Reason (optional)' })
  await expect(dialog.getByLabel('Reason (optional)', { exact: true })).toHaveValue('')
  await expect(dialog.getByRole('button', { name: 'Kick', exact: true })).toBeEnabled()
  await dialog.getByRole('button', { name: 'Kick', exact: true }).click()
  await expect(dialog).toBeHidden()
  await expect(page.locator('.member-button').filter({ hasText: 'Noah' })).toHaveCount(0)
})

test('profile name stays unset unless chosen, and clearing it restores the network name', async ({
  page,
}) => {
  const saves: Record<string, unknown>[] = []
  page.on('request', (request) => {
    if (request.method() === 'PATCH' && request.url().endsWith('/api/v1/profile'))
      saves.push(request.postDataJSON())
  })
  const nameInput = page.getByRole('textbox', { name: 'Display name', exact: true })
  const openProfile = async () => {
    await page.getByRole('button', { name: 'User settings', exact: true }).click()
    await page.getByRole('button', { name: 'Profile', exact: true }).click()
  }
  const save = async () => {
    await page.getByRole('button', { name: 'Save changes', exact: true }).click()
    await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  }
  await openProfile()
  await nameInput.fill('Chosen name')
  await save()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(page.locator('.self-profile')).toContainText('Chosen name')

  // Clearing saves the unset state; the network real name shows again, across a reload.
  await openProfile()
  await nameInput.fill('')
  await expect(page.locator('#profile-name-hint')).toContainText('Leave blank to use your IRC name')
  await save()
  expect(saves.at(-1)).toMatchObject({ displayName: '' })
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page.reload()
  await expect(page.locator('.self-profile')).toContainText('Karbowiak')
  await expect(page.locator('.self-profile')).not.toContainText('Chosen name')

  // Opening settings shows a blank name; saving only the bio or a new avatar sends no name.
  await openProfile()
  await expect(nameInput).toHaveValue('')
  await expect(page.getByRole('button', { name: 'Save changes', exact: true })).toBeDisabled()
  await page.getByRole('textbox', { name: 'About you', exact: true }).fill('Only the bio')
  await save()
  await page.getByLabel('Upload profile image').setInputFiles('public/avatars/jules.jpg')
  await expect(page.locator('.profile-avatar .avatar-image')).toBeVisible()
  await save()
  expect(saves.slice(-2).every((body) => !('displayName' in body))).toBe(true)
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page.reload()
  await openProfile()
  await expect(nameInput).toHaveValue('')
})
