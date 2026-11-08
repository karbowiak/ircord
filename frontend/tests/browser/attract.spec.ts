import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

test('/attract is hidden until /attr and makes your last message sparkle briefly', async ({
  page,
}) => {
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('look over here')
  await input.press('Enter')
  const mine = page.locator('.message-row.is-mine').filter({ hasText: 'look over here' })
  await expect(mine).toHaveCount(1)

  const suggestions = page.locator('.composer-suggestions')
  for (const typed of ['/', '/a', '/at', '/att']) {
    await input.fill(typed)
    await expect(suggestions.getByText('/attract')).toHaveCount(0)
  }
  await input.fill('/attr')
  await expect(suggestions.getByText('/attract')).toBeVisible()

  const scroll = page.locator('.timeline-scroll')
  const before = await scroll.evaluate((el) => el.scrollTop)
  // With the suggestion list open Enter completes the command, so submit with a trailing space.
  await input.fill('/attract ')
  await input.press('Enter')
  await expect(mine).toHaveClass(/is-attract/)
  await expect(mine.locator('.avatar')).toHaveClass(/is-attract/)
  expect(await scroll.evaluate((el) => el.scrollTop)).toBe(before)
  await expect(mine).not.toHaveClass(/is-attract/, { timeout: 4500 })
  await expect(input).toHaveValue('')
})

test('/attract keeps its own clock when the timeline re-renders mid-effect', async ({ page }) => {
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('steady now')
  await input.press('Enter')
  const mine = page.locator('.message-row.is-mine').filter({ hasText: 'steady now' })
  await expect(mine).toHaveCount(1)
  await input.fill('/attract ')
  await input.press('Enter')
  await expect(mine).toHaveClass(/is-attract/)
  const started = await page.evaluate(() => performance.now())
  await page.waitForTimeout(900)
  // Scrolling re-renders the timeline; the running animation must not jump ahead.
  await page.locator('.timeline-scroll').evaluate((el) => (el.scrollTop -= 1))
  await page.waitForTimeout(150)
  const drift = await page.evaluate((started) => {
    const shine = document
      .getAnimations()
      .find((a) => (a as CSSAnimation).animationName === 'attract-shine')
    if (!shine) return 'no animation'
    const timing = shine.effect!.getComputedTiming()
    const active = Number(timing.localTime) - Number(shine.effect!.getTiming().delay)
    return Math.abs(active - (performance.now() - started))
  }, started)
  expect(typeof drift === 'number' && drift < 200, `drift ${drift}`).toBe(true)
})

test('people below channel operator are told /attract is an unknown command', async ({ page }) => {
  await page.route('**/api/v1/bootstrap', async (route) => {
    const response = await route.fetch()
    const body = await response.json()
    for (const c of body.conversations) if (c.memberModes) c.memberModes.you = '+'
    await route.fulfill({ response, json: body })
  })
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('/attr')
  await expect(page.locator('.composer-suggestions')).toHaveCount(0)
  await input.fill('/attract')
  await input.press('Enter')
  await expect(page.locator('.toast')).toContainText(
    'Unknown command /attract. Use /quote to send it to the server as-is.',
  )
  await expect(page.locator('.is-attract')).toHaveCount(0)
})
