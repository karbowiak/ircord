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
  await expect(settings.locator('.about-facts dd').first()).toHaveText(/^[0-9A-Za-z-]{1,8}$/)
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
  // The settings toggle is a bubble-phase workspace shortcut that works even inside Settings;
  // only the game's stopPropagation keeps it from closing Settings mid-game.
  await page.keyboard.press('ControlOrMeta+Comma')
  await expect(settings).toBeVisible()
  await expect(game).toHaveAttribute('data-phase', 'playing')

  await page.keyboard.press('p')
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await page.keyboard.press('p')
  await expect(game).toHaveAttribute('data-phase', 'playing')
  // Leaving the window pauses.
  await page.evaluate(() => window.dispatchEvent(new Event('blur')))
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await game.getByRole('button', { name: 'Resume' }).first().click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  // A hidden tab pauses.
  await page.evaluate(() => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' })
    document.dispatchEvent(new Event('visibilitychange'))
  })
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await page.evaluate(() => {
    delete (document as { visibilityState?: unknown }).visibilityState
  })
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

test('Pause stays paused where clicking a button does not focus it', async ({ page }) => {
  const settings = await openAbout(page)
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  for (let i = 0; i < 5; i++) await logo.click()
  const game = settings.getByRole('region', { name: 'Hash Field' })
  await game.getByRole('button', { name: 'Start' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  // Safari and macOS Firefox: a mouse press blurs the focused element without focusing the button.
  await page.evaluate(() =>
    document.addEventListener(
      'mousedown',
      (event) => {
        event.preventDefault()
        ;(document.activeElement as HTMLElement | null)?.blur()
      },
      true,
    ),
  )
  await game.getByRole('button', { name: 'Pause', exact: true }).click()
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await page.waitForTimeout(100)
  await expect(game).toHaveAttribute('data-phase', 'paused')
})

test('the logo press count starts again after a pause', async ({ page }) => {
  const settings = await openAbout(page)
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  for (let i = 0; i < 4; i++) await logo.click()
  await page.waitForTimeout(1700)
  await logo.click()
  await expect(settings.getByRole('region', { name: 'Hash Field' })).toHaveCount(0)
  for (let i = 0; i < 4; i++) await logo.click()
  await expect(settings.getByRole('region', { name: 'Hash Field' })).toBeVisible()
})

test('Hash Field plays under reduced motion', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const settings = await openAbout(page)
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  await expect(logo).toHaveCSS('transition-duration', '0s')
  for (let i = 0; i < 5; i++) await logo.click()
  const game = settings.getByRole('region', { name: 'Hash Field' })
  await game.getByRole('button', { name: 'Start' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  await page.keyboard.down('Space')
  await page.keyboard.down('ArrowLeft')
  await page.waitForTimeout(1500)
  await page.keyboard.up('ArrowLeft')
  await page.keyboard.up('Space')
  await expect(game).toHaveAttribute('data-phase', /playing|over/)
  await page.keyboard.press('p')
  await expect(game).toHaveAttribute('data-phase', /paused|over/)
})

test('Hash Field stops its frame loop and listeners when it closes', async ({ page }) => {
  await page.addInitScript(() => {
    const w = window as unknown as { __frames: number; __listeners: number }
    w.__frames = 0
    w.__listeners = 0
    const request = window.requestAnimationFrame.bind(window)
    window.requestAnimationFrame = (callback) =>
      request((time) => {
        w.__frames++
        callback(time)
      })
    // Net count of window blur and document visibilitychange listeners.
    for (const [target, type] of [
      [window, 'blur'],
      [document, 'visibilitychange'],
    ] as const) {
      const add = target.addEventListener.bind(target)
      const remove = target.removeEventListener.bind(target)
      target.addEventListener = ((name: string, ...rest: [never, never]) => {
        if (name === type) w.__listeners++
        return add(name, ...rest)
      }) as typeof target.addEventListener
      target.removeEventListener = ((name: string, ...rest: [never, never]) => {
        if (name === type) w.__listeners--
        return remove(name, ...rest)
      }) as typeof target.removeEventListener
    }
  })
  await page.reload()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  const settings = await openAbout(page)
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  const listeners = () =>
    page.evaluate(() => (window as unknown as { __listeners: number }).__listeners)
  const before = await listeners()
  for (let i = 0; i < 5; i++) await logo.click()
  const game = settings.getByRole('region', { name: 'Hash Field' })
  await game.getByRole('button', { name: 'Start' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  expect(await listeners()).toBe(before + 2)
  const frames = () => page.evaluate(() => (window as unknown as { __frames: number }).__frames)
  await expect.poll(frames).toBeGreaterThan(5)
  // Take the focus-loss pause out of the way, so only the unmount cleanup can stop the loop.
  await page.evaluate(() =>
    document
      .querySelector('.hash-field')!
      .addEventListener('focusout', (event) => event.stopImmediatePropagation(), true),
  )
  await game.getByRole('button', { name: 'Back to About' }).click()
  await expect(game).toHaveCount(0)
  // Let anything still scheduled run once, then the count must stand still.
  await page.waitForTimeout(100)
  const settled = await frames()
  await page.waitForTimeout(400)
  expect(await frames()).toBe(settled)
  expect(await listeners()).toBe(before)
})
