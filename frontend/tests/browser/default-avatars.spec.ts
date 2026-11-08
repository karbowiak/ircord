import { test, expect, type Page } from '@playwright/test'
import { signIn, testPassword, uniqueUsername } from './helpers'

const picker = (page: Page) => page.getByRole('radiogroup', { name: 'Default profile pictures' })
/** Clicks an avatar in the grid, as a person would. */
const pick = (page: Page, id: string) =>
  picker(page).locator(`label:has(input[value="${id}"])`).click()
const checkedFile = async (page: Page) =>
  (await picker(page).getByRole('radio', { checked: true }).getAttribute('value')) ?? ''

test('sign-up suggests the same default avatar for the same username and saves the choice', async ({
  page,
  baseURL,
}) => {
  const saves: Record<string, unknown>[] = []
  page.on('request', (request) => {
    if (request.url().endsWith('/api/v1/profile') && request.method() === 'PATCH')
      saves.push(request.postDataJSON())
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await expect(picker(page).getByRole('radio')).toHaveCount(32)
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveCount(0)

  // The suggestion follows the username as it's typed, ignoring case.
  const name = page.getByLabel('Username', { exact: true })
  await name.fill('alice')
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveAttribute(
    'value',
    'avatar-031',
  )
  await expect(page.getByRole('radio', { name: /\(suggested\)/ })).toHaveAttribute(
    'value',
    'avatar-031',
  )
  await name.fill('mika')
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveAttribute(
    'value',
    'avatar-004',
  )
  await name.fill('ALICE')
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveAttribute(
    'value',
    'avatar-031',
  )

  // The same username gives the same suggestion on a fresh page.
  const username = uniqueUsername()
  await name.fill(username)
  const suggested = await checkedFile(page)
  expect(suggested).toMatch(/^avatar-\d{3}$/)
  await page.reload()
  await page.getByRole('button', { name: 'Create an account' }).click()
  await page.getByLabel('Username', { exact: true }).fill(username)
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveAttribute(
    'value',
    suggested,
  )

  // Picking another one sticks while the name changes further.
  const other = suggested === 'avatar-003' ? 'avatar-004' : 'avatar-003'
  await pick(page, other)
  await page.getByLabel('Username', { exact: true }).fill(`${username}x`)
  await page.getByLabel('Username', { exact: true }).fill(username)
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveAttribute('value', other)
  await expect(page.locator('.signup-picture .avatar-image')).toHaveAttribute(
    'src',
    `/avatars/defaults/${other}.png`,
  )

  await expect(page.getByText(`${username} is available`)).toBeVisible()
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  await page.getByRole('button', { name: 'Create account', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  const url = `${new URL(baseURL!).origin}/avatars/defaults/${other}.png`
  expect(saves).toEqual([{ bio: '', avatarUrl: url }])
  await expect(page.locator('.self-profile .avatar-image')).toHaveAttribute('src', url)
})

test('sign-up without a picture saves none', async ({ page }) => {
  const saves: unknown[] = []
  page.on('request', (request) => {
    if (request.url().endsWith('/api/v1/profile') && request.method() === 'PATCH')
      saves.push(request.postDataJSON())
  })
  const username = uniqueUsername()
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await page.getByLabel('Username', { exact: true }).fill(username)
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveCount(1)
  await page.getByRole('button', { name: 'No picture', exact: true }).click()
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveCount(0)
  await expect(page.getByText(`${username} is available`)).toBeVisible()
  await page.getByLabel('Password', { exact: true }).fill(testPassword)
  await page.getByRole('button', { name: 'Create account', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  expect(saves).toEqual([])
  await expect(page.locator('.self-profile .avatar-image')).toHaveCount(0)
})

test('profile settings switch between defaults, uploads and no picture', async ({
  page,
  baseURL,
}) => {
  await signIn(page)
  const saves: Record<string, unknown>[] = []
  page.on('request', (request) => {
    if (request.url().endsWith('/api/v1/profile') && request.method() === 'PATCH')
      saves.push(request.postDataJSON())
  })
  const open = async () => {
    await page.getByRole('button', { name: 'User settings', exact: true }).click()
    await page.getByRole('button', { name: 'Profile', exact: true }).click()
  }
  const save = async () => {
    await page.getByRole('button', { name: 'Save changes', exact: true }).click()
    await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  }
  await open()
  // The sign-up suggestion was saved, and shows as the chosen default.
  const initial = await checkedFile(page)
  expect(initial).toMatch(/^avatar-\d{3}$/)
  const next = initial === 'avatar-031' ? 'avatar-032' : 'avatar-031'
  await pick(page, next)
  await expect(page.locator('.profile-avatar .avatar-image')).toHaveAttribute(
    'src',
    `/avatars/defaults/${next}.png`,
  )
  await save()
  const url = `${new URL(baseURL!).origin}/avatars/defaults/${next}.png`
  expect(saves.at(-1)).toMatchObject({ avatarUrl: url })
  expect(saves.at(-1)).not.toHaveProperty('avatarDataUrl')
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await page.reload()
  await expect(page.locator('.self-profile .avatar-image')).toHaveAttribute('src', url)
  await open()
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveAttribute('value', next)

  // Uploading your own replaces the default; the picker then shows nothing chosen.
  await page.getByLabel('Upload profile image').setInputFiles('public/avatars/jules.jpg')
  await expect(picker(page).getByRole('radio', { checked: true })).toHaveCount(0)
  await save()
  expect(saves.at(-1)).toHaveProperty('avatarDataUrl')
  expect(saves.at(-1)).not.toHaveProperty('avatarUrl')

  // Removing still works.
  await page.getByRole('button', { name: 'Remove image', exact: true }).click()
  await save()
  expect(saves.at(-1)).toMatchObject({ avatarDataUrl: null })
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(page.locator('.self-profile .avatar-image')).toHaveCount(0)
})

test('avatars added to the manifest are offered with no code change', async ({ page }) => {
  await page.route('**/avatars/defaults/manifest.json', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    body.avatars.push({ id: 'avatar-033', file: 'avatar-001.png', category: 'extra' })
    await route.fulfill({ response, json: body })
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await expect(picker(page).getByRole('radio')).toHaveCount(33)
  await expect(picker(page).getByRole('radio', { name: 'Default avatar 33' })).toBeVisible()
})

test('without the avatar list, sign-up offers only an upload', async ({ page }) => {
  await page.route('**/avatars/defaults/manifest.json', (route) => route.fulfill({ status: 404 }))
  await page.goto('/')
  await page.getByRole('button', { name: 'Create an account' }).click()
  await page.getByLabel('Username', { exact: true }).fill(uniqueUsername())
  await expect(page.locator('.signup-picture')).toContainText('Upload one if you like')
  await expect(picker(page)).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Upload your own', exact: true })).toBeVisible()
})
