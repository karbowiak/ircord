import { test, expect } from '@playwright/test'
import { signIn } from './helpers'

// Large channels stay virtualized (#13). A layout that lets a column grow to its content makes
// the virtualizer render every row, so this checks the rendered count against the roster size.
test('a channel with thousands of people renders only the visible rows', async ({ page }) => {
  await signIn(page)
  const load = await page.request.post('/api/v1/_demo/busy', {
    data: { members: 2000, history: 1000, rate: 0, churn: 0, seconds: 10 },
  })
  expect(load.ok()).toBe(true)
  const link = page.locator('.conversation-link').filter({ hasText: /^busy/ })
  await link.click()
  await expect(page.getByRole('combobox', { name: 'Message #busy' })).toBeVisible()
  await expect(page.locator('.member-group-heading').first()).toBeVisible()
  const viewport = page.viewportSize()!.height
  const sidebar = (await page.locator('.member-sidebar').boundingBox())!
  expect(sidebar.height).toBeLessThanOrEqual(viewport)
  expect(await page.locator('.virtual-members > *').count()).toBeLessThan(100)
  expect(await page.locator('.virtual-messages > *').count()).toBeLessThan(100)
  // The roster is still all there: filtering finds someone far down the list.
  await page.getByRole('textbox', { name: 'Filter members' }).fill('user01999')
  await expect(
    page.locator('.member-sidebar').getByText('User 1999', { exact: true }),
  ).toBeVisible()
})
