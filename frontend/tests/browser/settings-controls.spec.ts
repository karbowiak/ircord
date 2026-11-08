import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

async function section(page: Page, name: string) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.locator('.settings-nav').getByRole('button', { name, exact: true }).click()
}

test('profile fields retain IME composition, textarea content and native Enter submission', async ({
  page,
}) => {
  await signIn(page)
  await section(page, 'Profile')
  const name = page.getByRole('textbox', { name: 'Display name', exact: true })
  const bio = page.getByRole('textbox', { name: 'About you', exact: true })
  const save = page.getByRole('button', { name: 'Save changes', exact: true })
  const changes: Record<string, unknown>[] = []
  page.on('request', (request) => {
    if (request.method() === 'PATCH' && request.url().endsWith('/api/v1/profile'))
      changes.push(request.postDataJSON())
  })

  await expect(save).toBeDisabled()
  await name.evaluate((element: HTMLInputElement) => {
    element.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
    element.value = '日本語'
    element.dispatchEvent(
      new InputEvent('input', { bubbles: true, data: '日本語', isComposing: true }),
    )
  })
  await expect(name).toHaveValue('日本語')
  await expect(save).toBeDisabled()
  await name.evaluate((element) =>
    element.dispatchEvent(
      new CompositionEvent('compositionend', { bubbles: true, data: '日本語' }),
    ),
  )
  await expect(save).toBeEnabled()
  await bio.fill('First line\nÆbler, 日本語 and ☕')
  await expect(bio).toHaveAttribute('maxlength', '300')
  await name.press('Enter')
  await expect(page.getByText('Profile saved', { exact: true })).toBeVisible()
  expect(changes).toEqual([
    { displayName: '日本語', bio: 'First line\nÆbler, 日本語 and ☕', nameColor: null },
  ])
})

test('font and layout cards follow keyboard focus and keep exactly one choice selected', async ({
  page,
}) => {
  await signIn(page)
  await section(page, 'Appearance')
  const fonts = page.getByRole('radiogroup', { name: 'Interface typeface', exact: true })
  const original = fonts.getByRole('radio', { checked: true })
  const oldValue = await original.getAttribute('value')
  await original.focus()
  await page.keyboard.press('ArrowRight')
  const focused = fonts.locator('[role="radio"]:focus')
  await expect(focused).toHaveAttribute('aria-checked', 'true')
  expect(await focused.getAttribute('value')).not.toBe(oldValue)
  await expect(fonts.getByRole('radio', { checked: true })).toHaveCount(1)

  await page
    .locator('.settings-nav')
    .getByRole('button', { name: 'Messages & media', exact: true })
    .click()
  const layouts = page.getByRole('radiogroup', { name: 'Message layout', exact: true })
  await layouts.getByRole('radio', { checked: true }).focus()
  await page.keyboard.press('End')
  await expect(layouts.getByRole('radio').last()).toBeFocused()
  await expect(layouts.getByRole('radio').last()).toHaveAttribute('aria-checked', 'true')
  await page.keyboard.press('Home')
  await expect(layouts.getByRole('radio').first()).toBeFocused()
  await expect(layouts.getByRole('radio').first()).toHaveAttribute('aria-checked', 'true')
  await expect(layouts.getByRole('radio', { checked: true })).toHaveCount(1)
})

test('a controlled switch toggles with Space and Enter and keeps its setting description', async ({
  page,
}) => {
  await signIn(page)
  await section(page, 'Messages & media')
  const control = page.getByRole('switch', { name: 'Coloured names', exact: true })
  await expect(control).toHaveAccessibleDescription('A stable colour for each person.')
  const before = await control.getAttribute('aria-checked')
  await control.focus()
  await page.keyboard.press('Space')
  await expect(control).toHaveAttribute('aria-checked', before === 'true' ? 'false' : 'true')
  await page.keyboard.press('Enter')
  await expect(control).toHaveAttribute('aria-checked', before!)
  await expect(control).toBeFocused()
})
