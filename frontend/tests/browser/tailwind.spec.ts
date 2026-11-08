import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

async function tokenColors(page: Page) {
  return page.evaluate(() => {
    const utility = document.createElement('span')
    const variables = document.createElement('span')
    utility.className = 'bg-primary text-primary-foreground'
    variables.style.backgroundColor = 'var(--accent)'
    variables.style.color = 'var(--accent-ink)'
    document.body.append(utility, variables)
    const actual = getComputedStyle(utility)
    const expected = getComputedStyle(variables)
    const result = {
      actual: [actual.backgroundColor, actual.color],
      expected: [expected.backgroundColor, expected.color],
    }
    utility.remove()
    variables.remove()
    return result
  })
}

test('semantic utilities follow the saved theme, including before the application starts', async ({
  page,
}) => {
  await signIn(page)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  for (const mode of ['Dark', 'Light']) {
    await page
      .getByRole('dialog', { name: 'Settings' })
      .getByRole('radio', { name: mode, exact: true })
      .click()
    await expect(page.locator('html')).toHaveAttribute('data-theme', mode.toLowerCase())
    const colors = await tokenColors(page)
    expect(colors.actual).toEqual(colors.expected)
  }
  // The inline boot script must restore the same variables without Vue or a theme plugin.
  await page.route('**/_nuxt/*.js', (route) => route.abort())
  await page.reload({ waitUntil: 'domcontentloaded' })
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
  await expect(page.locator('#ircord-theme')).toHaveCount(1)
  await expect
    .poll(async () => {
      const colors = await tokenColors(page)
      return colors.actual.join('|') === colors.expected.join('|')
    })
    .toBe(true)
})

test('select overrides and duration fields keep their compact native geometry', async ({
  page,
}) => {
  await signIn(page)
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  const nav = page.locator('.settings-nav')
  await nav.getByRole('button', { name: 'Profile', exact: true }).click()
  const network = page.getByRole('combobox', { name: 'Network', exact: true })
  await expect(network).toHaveCSS('min-height', '30px')
  await expect(network).toHaveCSS('height', '30px')
  expect(await network.evaluate((el) => el.getBoundingClientRect().width)).toBeLessThan(250)
  await network.focus()
  await page.keyboard.press('ArrowDown')
  await expect(page.getByRole('listbox')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(network).toBeFocused()

  await nav.getByRole('button', { name: 'Notifications', exact: true }).click()
  const duration = page.locator('.duration-input input')
  await expect(duration).toBeVisible()
  const size = await duration.evaluate((el) => ({
    width: el.getBoundingClientRect().width,
    font: parseFloat(getComputedStyle(el).fontSize),
  }))
  expect(size.width).toBeCloseTo(size.font * 5.5, 0)
})
