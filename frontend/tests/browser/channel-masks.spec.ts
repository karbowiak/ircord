import { expect, test } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('ban lists explain masks and show what Add will send', async ({ page }) => {
  await page.locator('.channel-name-button').click()
  const dialog = page.getByRole('dialog', { name: '#general settings' })
  await dialog.getByRole('button', { name: 'Bans', exact: true }).click()
  await expect(dialog.locator('.list-purpose')).toContainText('Ban exceptions override bans')
  const field = dialog.getByLabel('Add ban')
  const add = dialog.getByRole('button', { name: 'Add ban' })
  const preview = dialog.locator('#mask-preview')
  await expect(field).toHaveAttribute('aria-describedby', 'mask-short-help mask-preview')

  await field.fill('troll')
  await expect(preview).toContainText(
    'Adds troll!*@* for “troll”. Matches anyone with the nick troll.',
  )
  await field.fill('*')
  await expect(preview).toContainText('This matches everyone, including you.')
  await expect(preview).toContainText('(you)')
  await field.fill('two words')
  await expect(preview).toContainText('Masks can’t contain spaces.')
  await expect(field).toHaveAttribute('aria-invalid', 'true')
  await expect(add).toBeDisabled()

  // Examples fill the field only; the list changes on Add.
  const entries = dialog.locator('.mask-list li')
  const before = await entries.count()
  await dialog.getByText('How masks work, with examples').click()
  await expect(dialog.locator('.mask-help')).toContainText(
    'Libera Chat also takes account masks: $a:name',
  )
  await dialog.getByRole('button', { name: 'Use the example *!*@203.0.113.7' }).click()
  await expect(field).toHaveValue('*!*@203.0.113.7')
  await expect(field).toBeFocused()
  await expect(entries).toHaveCount(before)
  await dialog.getByRole('button', { name: 'Use the example $a:alice' }).click()
  await expect(preview).toContainText('Anyone logged in to the account alice')
  await add.click()
  await expect(dialog.locator('.mask-list')).toContainText('$a:alice')

  await page.setViewportSize({ width: 390, height: 844 })
  await dialog.screenshot({ path: 'test-results/mask-help-mobile.png' })
})

test('exception lists say what they override', async ({ page }) => {
  await page.locator('.channel-name-button').click()
  const dialog = page.getByRole('dialog', { name: '#general settings' })
  await dialog.getByRole('button', { name: 'Ban exceptions', exact: true }).click()
  await expect(dialog.locator('.list-purpose')).toContainText('lets them past bans')
  await dialog.getByRole('button', { name: 'Invite exceptions', exact: true }).click()
  await expect(dialog.locator('.list-purpose')).toContainText('a ban still keeps them out')
  await expect(dialog).toContainText('Invite-only (+i) is off')
})
