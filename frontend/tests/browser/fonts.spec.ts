import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const openSettings = async (page: Page) => {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  return page.getByRole('dialog', { name: 'Settings' })
}
const openSection = (page: Page, name: string) =>
  page
    .getByRole('dialog', { name: 'Settings' })
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name, exact: true })
    .click()
const cssVar = (page: Page, name: string) =>
  page.evaluate((name) => getComputedStyle(document.documentElement).getPropertyValue(name), name)
/** True once the named face has actually downloaded for this text, not just fallen back. */
const loaded = (page: Page, family: string, text: string) =>
  page.evaluate(
    async ([family, text]) => {
      await document.fonts.load(`16px '${family}'`, text)
      return [...document.fonts].some(
        (face) => face.family.replace(/['"]/g, '') === family && face.status === 'loaded',
      )
    },
    [family, text] as const,
  )

test('each selector offers eight fonts and new choices load, persist and stay independent', async ({
  page,
}) => {
  const settings = await openSettings(page)
  const messageFonts = settings.getByRole('radiogroup', { name: 'Message font' })
  const interfaceFonts = settings.getByRole('radiogroup', { name: 'Interface typeface' })
  await openSection(page, 'Appearance')
  await expect(interfaceFonts.getByRole('radio')).toHaveCount(8)

  await interfaceFonts.getByRole('radio', { name: /^Aa ?Source Sans/ }).click()
  await expect.poll(() => cssVar(page, '--font-ui')).toContain('Source Sans 3 Variable')
  // "Interface" message text follows the chosen UI typeface.
  await expect(page.locator('.message-text').first()).toHaveCSS(
    'font-family',
    /^"?Source Sans 3 Variable/,
  )
  expect(await loaded(page, 'Source Sans 3 Variable', 'Settings æøå')).toBe(true)

  await openSection(page, 'Messages & media')
  await expect(messageFonts.getByRole('radio')).toHaveCount(8)
  const danish = 'Ses vi på fredag? Æbler, øl'
  await expect(messageFonts.getByRole('radio').first()).toContainText(danish)
  for (const [label, family] of [
    ['Lexend', 'Lexend Variable'],
    ['Literata', 'Literata Variable'],
    ['Atkinson Mono', 'Atkinson Hyperlegible Mono Variable'],
  ] as const) {
    const choice = messageFonts.getByRole('radio').filter({
      has: page.getByText(label, { exact: true }),
    })
    await choice.click()
    await expect(choice).toHaveAttribute('aria-checked', 'true')
    await expect.poll(() => cssVar(page, '--font-message')).toContain(family)
    await expect.poll(() => cssVar(page, '--font-ui')).toContain('Source Sans 3 Variable')
    expect(await loaded(page, family, danish), `${family} loads`).toBe(true)
  }

  // Saved to the account, not only this device: forget the device copy before reloading.
  await openSection(page, 'Appearance')
  await expect(settings.getByRole('status').filter({ hasText: 'account' })).toHaveText(
    'Synced to your account',
  )
  await page.evaluate(() => {
    localStorage.removeItem('ircord.appearance')
    localStorage.removeItem('ircord.appearance.compiled')
  })
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await expect
    .poll(() => cssVar(page, '--font-message'))
    .toContain('Atkinson Hyperlegible Mono Variable')
  await expect.poll(() => cssVar(page, '--font-ui')).toContain('Source Sans 3 Variable')
  const body = page.locator('.message-text').first()
  await expect(body).toHaveCSS('font-family', /Atkinson Hyperlegible Mono Variable/)
})
