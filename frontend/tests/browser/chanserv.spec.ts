import { expect, test, type Page } from '@playwright/test'
import { chooseOption, signIn } from './helpers'

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

  await panel.getByRole('combobox', { name: 'Account to add' }).fill('noah')
  await chooseOption(page, panel.getByRole('combobox', { name: 'Role to give' }), 'Voice')
  await panel.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(rows.filter({ hasText: 'noah' })).toHaveCount(1)
  await expect(panel.getByRole('combobox', { name: 'Role for noah' })).toHaveText('Voice')

  await panel.getByRole('combobox', { name: 'Account to add' }).fill('nobody')
  await panel.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(panel.getByRole('alert')).toHaveText('Account does not exist.')

  await chooseOption(page, panel.getByRole('combobox', { name: 'Role for mika' }), 'Admin')
  await panel.getByRole('button', { name: 'Remove sage' }).click()
  await page
    .getByRole('dialog', { name: 'Remove sage?' })
    .getByRole('button', { name: 'Remove' })
    .click()
  await expect(rows.filter({ hasText: 'sage' })).toHaveCount(0)
  await expect(panel.getByRole('combobox', { name: 'Role for mika' })).toHaveText('Admin')

  await panel.getByRole('combobox', { name: 'Transfer to account' }).fill('mika')
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
  await expect(panel.getByRole('combobox', { name: 'Account to add' })).toHaveCount(0)
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

test('registration uses the observed account and recovers after a service outage', async ({ page }) => {
  let calls = 0
  await page.route('**/api/v1/conversations/design/registration', route => {
    if (calls++ === 0) return route.fulfill({ status: 504, json: { detail: 'ChanServ did not answer in time.' } })
    return route.continue()
  })
  const panel = await registration(page, 'design')
  await expect(panel.getByRole('alert')).toContainText('did not answer')
  await panel.getByRole('button', { name: 'Try again' }).click()
  await expect(panel).toContainText('#design isn’t registered')
  await page.keyboard.press('Escape')
  await page.route('**/api/v1/bootstrap', async route => {
    const response = await route.fetch()
    const body = await response.json()
    body.networks = body.networks.map((n: { id: string }) => ({ ...n, account: '', auth: { method: 'sasl-plain', account: 'saved-login-only', passwordSet: true } }))
    await route.fulfill({ response, json: body })
  })
  await page.reload()
  const signedOut = await registration(page, 'design')
  await expect(signedOut).toContainText('account to register channels')
  await expect(signedOut.getByRole('button', { name: 'Register #design' })).toBeDisabled()
})

test('private live service updates refresh an open access list', async ({ page }) => {
  const panel = await registration(page, 'general')
  await expect(panel.locator('.access-list li')).toHaveCount(3)
  // A mutation in another tab on this account must refresh this view without polling.
  const result = await page.context().request.put('/api/v1/conversations/general/access/noah', { data: { mode: 'v' } })
  expect(result.ok()).toBeTruthy()
  await expect(panel.getByRole('combobox', { name: 'Role for noah' })).toHaveText('Voice')
})

test('account fields suggest people by their services account, never their nick', async ({ page }) => {
  // jules uses the account julia, lin isn't logged in, and mika has a second session.
  await page.route('**/api/v1/bootstrap', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    for (const m of body.members) {
      if (m.id === 'jules') m.account = 'julia'
      if (m.id === 'lin') m.account = ''
    }
    body.members.push({ ...body.members.find((m: { id: string }) => m.id === 'mika'), id: 'mika-phone', nick: 'mika_phone' })
    const general = body.conversations.find((c: { id: string }) => c.id === 'general')
    general.memberIds.push('mika-phone')
    await route.fulfill({ response, json: body })
  })
  await page.reload()
  const panel = await registration(page, 'general')
  const rows = panel.locator('.access-list li')
  await expect(rows).toHaveCount(3)
  const field = panel.getByRole('combobox', { name: 'Account to add' })
  const list = page.getByRole('listbox', { name: 'People in #general' })

  // A nick finds the account; choosing it fills the field and does nothing else.
  await field.fill('jul')
  await expect(field).toHaveAttribute('aria-expanded', 'true')
  const julia = list.getByRole('option', { name: /julia/ })
  await expect(julia).toContainText('in #general as jules')
  await field.press('ArrowDown')
  await expect(field).toHaveAttribute('aria-activedescendant', (await julia.getAttribute('id'))!)
  await field.press('Enter')
  await expect(field).toHaveValue('julia')
  await expect(list).toBeHidden()
  await expect(rows).toHaveCount(3)
  await expect(panel.locator('.account-target').first()).toContainText('Acts on the account julia')

  // Typing the nick itself points at the account it belongs to.
  await field.fill('jules')
  await expect(panel.locator('.account-target').first()).toContainText('jules is logged in as julia')

  // Someone without an account is explained and can't be chosen.
  await field.fill('lin')
  const lin = list.getByRole('option', { name: /lin/ })
  await expect(lin).toHaveAttribute('aria-disabled', 'true')
  await expect(lin).toContainText('Not logged in to an account')
  await expect(panel.locator('.account-target').first()).toContainText('lin isn’t logged in')

  // Two sessions on one account are one suggestion.
  await field.fill('mika')
  await expect(list.getByRole('option')).toHaveCount(1)
  await expect(list.getByRole('option')).toContainText('in #general as mika_phone')
  await expect(list.getByRole('option')).toContainText('Operator now')
  await page.screenshot({ path: 'test-results/account-combobox-desktop.png' })

  // Escape closes the suggestions, not the dialog.
  await field.press('Escape')
  await expect(list).toBeHidden()
  await expect(panel).toBeVisible()

  // Pointer selection, then Add acts on the account shown.
  await field.fill('noa')
  await list.getByRole('option', { name: /noah/ }).click()
  await expect(field).toHaveValue('noah')
  await expect(field).toBeFocused()
  await expect(rows).toHaveCount(3)
  await panel.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(rows.filter({ hasText: 'noah' })).toHaveCount(1)
  await expect(field).toHaveValue('')

  // Choosing a new founder never starts the transfer by itself.
  const transfer = panel.getByRole('combobox', { name: 'Transfer to account' })
  await transfer.fill('jul')
  await transfer.press('ArrowDown')
  await transfer.press('Enter')
  await expect(transfer).toHaveValue('julia')
  await expect(page.getByRole('dialog', { name: /Transfer #general/ })).toHaveCount(0)

  await page.setViewportSize({ width: 390, height: 844 })
  await panel.screenshot({ path: 'test-results/account-combobox-mobile.png' })
})

test.describe('on a touch screen', () => {
  test.use({ hasTouch: true, viewport: { width: 390, height: 844 } })
  test('tapping a suggestion fills the account without adding it', async ({ page }) => {
    await page.locator('.channel-name-button').tap()
    const panel = page.getByRole('dialog', { name: '#general settings' })
    await panel.getByRole('button', { name: 'Registration', exact: true }).tap()
    const field = panel.getByRole('combobox', { name: 'Account to add' })
    await field.tap()
    await field.fill('no')
    await page.getByRole('listbox').getByRole('option', { name: /noah/ }).tap()
    await expect(field).toHaveValue('noah')
    await expect(page.getByRole('listbox')).toBeHidden()
    await expect(panel.locator('.access-list li')).toHaveCount(3)
  })
})
