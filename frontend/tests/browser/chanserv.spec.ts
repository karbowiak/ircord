import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

async function registration(page: Page, channel: string) {
  await page
    .locator('.conversation-link')
    .filter({ hasText: channel })
    .first()
    .click({ button: 'right' })
  await page.getByRole('menuitem', { name: /Channel settings/ }).click()
  await page.getByRole('button', { name: 'Registration', exact: true }).click()
  return page.getByRole('dialog', { name: `#${channel} settings` })
}

test('the founder manages access and can unregister', async ({ page }) => {
  const panel = await registration(page, 'general')
  await expect(panel.locator('.channel-facts')).toContainText('(you)')
  const rows = panel.locator('.access-list li')
  await expect(rows).toHaveCount(3)
  // The founder's own row can't be changed or removed here.
  await expect(rows.first()).toContainText('Founder')
  await expect(rows.first().getByRole('button')).toHaveCount(0)

  await panel.getByRole('textbox', { name: 'Account to add' }).fill('noah')
  await panel.getByRole('combobox', { name: 'Role to give' }).selectOption('v')
  await panel.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(rows.filter({ hasText: 'noah' })).toHaveCount(1)
  await expect(panel.getByRole('combobox', { name: 'Role for noah' })).toHaveValue('v')

  await panel.getByRole('textbox', { name: 'Account to add' }).fill('nobody')
  await panel.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(panel.getByRole('alert')).toHaveText('Account does not exist.')

  await panel.getByRole('combobox', { name: 'Role for mika' }).selectOption('a')
  await panel.getByRole('button', { name: 'Remove sage' }).click()
  await page
    .getByRole('dialog', { name: 'Remove sage?' })
    .getByRole('button', { name: 'Remove' })
    .click()
  await expect(rows.filter({ hasText: 'sage' })).toHaveCount(0)
  await expect(panel.getByRole('combobox', { name: 'Role for mika' })).toHaveValue('a')

  await panel.getByRole('textbox', { name: 'Transfer to account' }).fill('mika')
  await panel.getByRole('button', { name: 'Transfer', exact: true }).click()
  await page
    .getByRole('dialog', { name: 'Transfer #general to mika?' })
    .getByRole('button', { name: 'Transfer' })
    .click()
  await expect(panel.getByRole('status')).toContainText('TRANSFER accept #general')

  await panel.getByRole('button', { name: 'Unregister #general' }).click()
  const confirm = page.getByRole('dialog', { name: 'Unregister #general?' })
  await expect(confirm.getByRole('button', { name: 'Unregister' })).toBeDisabled()
  await confirm.getByRole('textbox').fill('#general')
  await confirm.getByRole('button', { name: 'Unregister' }).click()
  await expect(panel).toContainText('#general isn’t registered')
  await expect(panel.getByRole('button', { name: 'Register #general' })).toBeEnabled()
})

test('others see the founder, and registering needs operator status', async ({ page }) => {
  let panel = await registration(page, 'development')
  await expect(panel.locator('.channel-facts')).toContainText('mika')
  await expect(panel).toContainText('Only the founder and people with admin or operator access')
  await expect(panel.getByRole('textbox', { name: 'Account to add' })).toHaveCount(0)
  await expect(panel.getByRole('button', { name: /Unregister/ })).toHaveCount(0)
  await page.keyboard.press('Escape')

  panel = await registration(page, 'design')
  await expect(panel.getByRole('button', { name: 'Register #design' })).toBeDisabled()
  await expect(panel).toContainText('You need operator status (@) in #design')
})

test('registers a channel you operate, and explains unsupported networks', async ({ page }) => {
  await page.getByRole('button', { name: 'Join a channel', exact: true }).click()
  await page.getByRole('textbox', { name: 'Channel name' }).fill('fresh-room')
  await page.getByRole('button', { name: 'Join', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #fresh-room' })).toBeEnabled()
  let panel = await registration(page, 'fresh-room')
  await panel.getByRole('button', { name: 'Register #fresh-room' }).click()
  await expect(panel.getByRole('status')).toHaveText('#fresh-room is registered to you.')
  await expect(panel.locator('.channel-facts')).toContainText('(you)')
  await expect(panel.locator('.access-list li')).toHaveCount(1)
  await page.keyboard.press('Escape')

  await page.getByRole('button', { name: 'OFTC', exact: true }).click()
  panel = await registration(page, 'community')
  await expect(panel).toContainText('doesn’t have a ChanServ that IRCord can manage')
  await expect(panel.getByRole('button', { name: /Register/ })).toHaveCount(0)
})
