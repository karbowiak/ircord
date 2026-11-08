import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

async function openAbout(page: Page) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'About' })
    .click()
  return page.getByRole('dialog', { name: 'Settings' })
}

test('Settings → About describes this build and links to the project', async ({ page }) => {
  const settings = await openAbout(page)
  await expect(settings.getByRole('heading', { name: 'IRCord', level: 4 })).toBeVisible()
  await expect(settings.getByRole('button', { name: 'IRCord' }).locator('img')).toHaveAttribute(
    'src',
    '/favicon.svg',
  )
  // The test server builds as v0.1.2 and runs the demo.
  await expect(settings.getByText('Version v0.1.2')).toBeVisible()
  await expect(settings.locator('.about-demo-badge')).toHaveText('Demo')
  await expect(settings.getByText('This is a demo: no IRC network is connected')).toBeVisible()
  await expect(settings.locator('.about-facts dd').nth(1)).toHaveText(/^[0-9A-Za-z-]{1,8}$/)
  await expect(settings.getByText('MIT · © 2026 Michael Karbowiak')).toBeVisible()

  const links = settings.getByRole('navigation', { name: 'IRCord links' })
  const repo = 'https://github.com/karbowiak/ircord'
  for (const [name, href] of [
    ['Source code', repo],
    ['Report a problem', `${repo}/issues`],
    ['All releases', `${repo}/releases`],
    ['MIT License', `${repo}/blob/main/LICENSE`],
  ] as const) {
    const link = links.getByRole('link', { name: new RegExp(`^${name}`) })
    await expect(link).toHaveAttribute('href', href)
    await expect(link).toHaveAttribute('target', '_blank')
    await expect(link).toHaveAttribute('rel', 'noopener noreferrer')
  }
  await links.getByRole('button', { name: /^Release notes/ }).click()
  await expect(settings.getByRole('heading', { name: 'Release notes' })).toBeFocused()
  await expect(settings.getByRole('region', { name: 'Release notes' })).toBeVisible()
})

test('Settings → About fits a phone screen', async ({ page }) => {
  const settings = await openAbout(page)
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(settings.getByText('Version v0.1.2')).toBeVisible()
  const content = settings.locator('.settings-content')
  expect(await content.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  // The game and its touch controls fit too.
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  for (let i = 0; i < 5; i++) await logo.click()
  await expect(settings.getByRole('button', { name: 'Back to About' })).toBeInViewport()
  expect(await content.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
})

test('the Hash Field Easter egg keeps its keys, pauses, and returns to About', async ({ page }) => {
  const composer = page.getByRole('combobox', { name: 'Message #general' })
  await composer.fill('a draft that must survive')
  const settings = await openAbout(page)
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  const game = settings.getByRole('region', { name: 'Hash Field' })

  // Opening About starts nothing; four presses are not enough.
  for (let i = 0; i < 4; i++) await logo.click()
  await expect(game).toHaveCount(0)
  await logo.press('Enter')
  await expect(game).toHaveAttribute('data-phase', 'ready')
  await expect(game.getByRole('button', { name: 'Start' })).toBeFocused()

  await game.getByRole('button', { name: 'Start' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  const field = game.getByRole('img', { name: 'Hash Field playfield' })
  await expect(field).toBeFocused()
  // Game keys and typing stay in the game: no composer, no workspace shortcuts.
  await page.keyboard.down('ArrowUp')
  await page.keyboard.press('Space')
  await page.keyboard.type('hello?')
  await page.keyboard.press('Enter')
  await page.keyboard.up('ArrowUp')
  await expect(field).toBeFocused()
  await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toHaveCount(0)

  await page.keyboard.press('p')
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await page.keyboard.press('p')
  await expect(game).toHaveAttribute('data-phase', 'playing')
  // Leaving the window pauses.
  await page.evaluate(() => window.dispatchEvent(new Event('blur')))
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await game.getByRole('button', { name: 'Resume' }).first().click()
  await expect(game).toHaveAttribute('data-phase', 'playing')

  // Escape returns to About without closing Settings, and focus goes back to the logo.
  await page.keyboard.press('Escape')
  await expect(game).toHaveCount(0)
  await expect(settings).toBeVisible()
  await expect(logo).toBeFocused()
  await expect(settings.getByText('Version v0.1.2')).toBeVisible()

  // The next Escape closes Settings as usual; the draft is untouched.
  await page.keyboard.press('Escape')
  await expect(settings).toHaveCount(0)
  await expect(composer).toHaveValue('a draft that must survive')
})

test('Hash Field stops its frame loop when it closes', async ({ page }) => {
  await page.addInitScript(() => {
    const w = window as unknown as { __frames: number }
    w.__frames = 0
    const request = window.requestAnimationFrame.bind(window)
    window.requestAnimationFrame = (callback) =>
      request((time) => {
        w.__frames++
        callback(time)
      })
  })
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  const settings = await openAbout(page)
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  for (let i = 0; i < 5; i++) await logo.click()
  const game = settings.getByRole('region', { name: 'Hash Field' })
  await game.getByRole('button', { name: 'Start' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  const frames = () => page.evaluate(() => (window as unknown as { __frames: number }).__frames)
  await expect.poll(frames).toBeGreaterThan(5)
  await game.getByRole('button', { name: 'Back to About' }).click()
  await expect(game).toHaveCount(0)
  // Let anything still scheduled run once, then the count must stand still.
  await page.waitForTimeout(100)
  const settled = await frames()
  await page.waitForTimeout(400)
  expect(await frames()).toBe(settled)
})
