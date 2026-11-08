import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'
import { insertScore, type ScoreEntry } from '../../shared/utils/hash-field-scores'

async function openGame(page: Page) {
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  const settings = page.getByRole('dialog', { name: 'Settings' })
  await settings
    .getByRole('navigation', { name: 'Settings' })
    .getByRole('button', { name: 'About' })
    .click()
  const logo = settings.getByRole('button', { name: 'IRCord', exact: true })
  for (let i = 0; i < 5; i++) await logo.click()
  return settings.getByRole('region', { name: 'Hash Field' })
}

test('the High scores link shows the shared table and pauses a running game', async ({ page }) => {
  await signIn(page)
  const game = await openGame(page)
  await game.getByRole('button', { name: 'Start' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')

  await game.getByRole('button', { name: 'High scores' }).click()
  await expect(game).toHaveAttribute('data-phase', 'paused')
  const table = game.getByRole('table')
  await expect(table.getByRole('row', { name: /Michael Karbowiak/ })).toContainText('97,909')
  await expect(table.getByRole('row', { name: /Peter Petermann/ })).toContainText('22,831')
  // Nothing in the table says where an entry came from.
  await expect(game.getByRole('region', { name: 'High scores' })).not.toContainText(
    /line|commit|contribut/i,
  )

  await page.keyboard.press('Escape')
  await expect(table).toHaveCount(0)
  await expect(game).toHaveAttribute('data-phase', 'paused')
  await game.getByRole('button', { name: 'Resume' }).first().click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
})

test('losing the last ship shows game over, takes a name and highlights the new entry', async ({
  page,
}) => {
  await signIn(page)
  const displayName = await page.evaluate(async () => {
    const snapshot = await (await fetch('/api/v1/bootstrap')).json()
    return snapshot.user.displayName as string
  })
  expect(displayName).not.toBe('')
  // A table with room, so whatever this game scores places. The demo server's own table and
  // validation are covered by the Go and unit tests; here the browser flow is under test.
  let rows: Omit<ScoreEntry, 'rank'>[] = [
    { name: 'Michael Karbowiak', score: 97909 },
    { name: 'Peter Petermann', score: 22831 },
  ]
  const submitted: { name: string; score: number }[] = []
  // The table load at game over waits until the test lets it through.
  let release = () => {}
  let held: Promise<void> | undefined
  const holdTable = () => (held = new Promise((resolve) => (release = resolve)))
  await page.route('**/api/v1/hash-field/scores', async (route) => {
    if (route.request().method() === 'POST') {
      const body = route.request().postDataJSON() as { name: string; score: number }
      submitted.push(body)
      const result = insertScore(rows, body.name, body.score)
      rows = result.items
      return route.fulfill({
        json: { placed: result.rank > 0, rank: result.rank, items: result.items },
      })
    }
    await held
    return route.fulfill({ json: { items: rows.map((row, i) => ({ ...row, rank: i + 1 })) } })
  })
  await page.clock.install()
  const game = await openGame(page)
  const heading = game.getByRole('heading', { name: 'Game over' })
  const name = game.getByRole('textbox', { name: 'Name for the high score table' })
  /** Spins and fires in place until the drifting hashes take all three ships. */
  async function loseAllShips() {
    await expect(game).toHaveAttribute('data-phase', 'playing')
    holdTable()
    await page.keyboard.down('ArrowLeft')
    await page.keyboard.down('Space')
    for (let i = 0; i < 120 && (await game.getAttribute('data-phase')) !== 'over'; i++)
      await page.clock.runFor(5_000)
    await expect(game).toHaveAttribute('data-phase', 'over')
    // The fire key still held, and pressed again, while the table loads skips nothing.
    await expect(heading).toBeFocused()
    await page.keyboard.up('Space')
    await page.keyboard.up('ArrowLeft')
    await page.keyboard.press('Space')
    await page.keyboard.press('Enter')
    await expect(game).toHaveAttribute('data-phase', 'over')
    release()
    await expect(name).toBeFocused()
    return Number((await game.locator('.hash-field-final').textContent())!.replaceAll(',', ''))
  }

  await game.getByRole('button', { name: 'Start' }).click()
  const final = await loseAllShips()
  expect(final).toBeGreaterThan(0)

  await expect(name).toHaveValue(displayName)
  await name.fill('  Ace   Pilot ')
  await page.keyboard.press('Enter')
  expect(submitted).toEqual([{ name: 'Ace Pilot', score: final }])
  await expect(name).toHaveCount(0)

  const table = game.getByRole('table')
  const current = table.locator('tr[aria-current="true"]')
  await expect(current).toHaveCount(1)
  await expect(current).toContainText('Ace Pilot')
  await expect(current).toContainText(final.toLocaleString('en-US'))
  await expect(table.getByRole('row', { name: /Michael Karbowiak/ })).toBeVisible()
  await expect(game.getByRole('button', { name: 'Play again' })).toBeFocused()

  // Play again ignores a press in its first moments, while the fire key may still be down.
  await page.clock.runFor(600)
  await game.getByRole('button', { name: 'Play again' }).click()
  await expect(game).toHaveAttribute('data-phase', 'playing')
  await expect(table).toHaveCount(0)

  // Escape in the name field cancels the entry instead of leaving the game.
  await loseAllShips()
  await page.keyboard.press('Escape')
  await expect(name).toHaveCount(0)
  await expect(game).toHaveAttribute('data-phase', 'over')
  await expect(game.getByRole('button', { name: 'Play again' })).toBeFocused()
  expect(submitted).toHaveLength(1)
})
