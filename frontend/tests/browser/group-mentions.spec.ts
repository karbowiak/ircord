import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

// In the demo's #general you are an operator (@) and Echo, who repeats what you send, is voiced.
const composer = (page: Page) => page.getByRole('combobox', { name: 'Message #general' })
async function send(page: Page, text: string) {
  await composer(page).fill(text)
  await page.keyboard.press('Escape')
  await composer(page).press('Enter')
}
const echoed = (page: Page, text: string) =>
  page.locator('.message-row').filter({ hasText: text }).last()

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

test('offers group mentions in @ autocomplete and shows them as mention pills', async ({
  page,
}) => {
  const input = composer(page)
  await input.click()
  await input.pressSequentially('@he')
  await expect(page.getByRole('option', { name: /@here/ })).toBeVisible()
  await page.keyboard.press('Tab')
  await expect(input).toHaveValue('@here ')
  await expect(page.getByRole('listbox')).toHaveCount(0)
  await input.fill('ping @op')
  await expect(page.getByRole('option', { name: /@operator/ })).toBeVisible()
  await page.keyboard.press('Enter')
  await expect(input).toHaveValue('ping @operator ')
  await input.press('Enter')
  await expect(
    page
      .locator('.message-row')
      .filter({ hasText: 'ping @operator' })
      .first()
      .locator('.inline-mention'),
  ).toHaveText('@operator')
})

test('role mentions match your own channel status, and the channel setting gates @here', async ({
  page,
}) => {
  // Role mentions are open to everyone: Echo (voiced) reaching operators includes you.
  await send(page, 'calling @operator now')
  await expect(echoed(page, 'calling @operator now')).toHaveClass(/is-mention/)
  await send(page, 'and @owner too')
  await expect(page.locator('.message-row').filter({ hasText: 'and @owner too' })).toHaveCount(2)
  await expect(echoed(page, 'and @owner too')).not.toHaveClass(/is-mention/)

  // Default: only operators may use @here; Echo is only voiced.
  await send(page, 'first @here')
  await expect(page.locator('.message-row').filter({ hasText: 'first @here' })).toHaveCount(2)
  await expect(echoed(page, 'first @here')).not.toHaveClass(/is-mention/)

  await page.locator('.channel-name-button').click()
  const dialog = page.getByRole('dialog', { name: '#general settings' })
  const gate = dialog.getByRole('group', {
    name: 'Who can use @here, @channel and @everyone in this channel',
  })
  await expect(gate.getByRole('button', { name: 'Operators' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await gate.getByRole('button', { name: 'Everyone' }).click()
  await expect(gate.getByRole('button', { name: 'Everyone' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
  await page.keyboard.press('Escape')
  await send(page, 'second @everyone')
  await expect(echoed(page, 'second @everyone')).toHaveClass(/is-mention/)

  await page.locator('.channel-name-button').click()
  await gate.getByRole('button', { name: 'Off' }).click()
  await expect(gate.getByRole('button', { name: 'Off' })).toHaveAttribute('aria-pressed', 'true')
  await page.keyboard.press('Escape')
  await send(page, 'third @channel')
  await expect(page.locator('.message-row').filter({ hasText: 'third @channel' })).toHaveCount(2)
  await expect(echoed(page, 'third @channel')).not.toHaveClass(/is-mention/)
})
