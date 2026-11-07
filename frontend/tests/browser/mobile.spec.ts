import { test, expect, devices } from '@playwright/test'
import { signIn } from './helpers'

test.use({ ...devices['iPhone 13'], defaultBrowserType: undefined })

import type { CDPSession, Page } from '@playwright/test'

// One DevTools session per page: a touch sequence must stay on the same session.
const sessions = new WeakMap<Page, Promise<CDPSession>>()
async function touch(page: Page, type: string, points: { x: number; y: number }[]) {
  if (!sessions.has(page)) sessions.set(page, page.context().newCDPSession(page))
  await (await sessions.get(page)!).send('Input.dispatchTouchEvent', { type, touchPoints: points })
}

test('phones get drawers, long-press menus and a bottom-sheet picker', async ({ page }) => {
  await signIn(page)
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
  await expect(page.locator('.conversation-sidebar')).toBeHidden()

  // Long-press a message: the message menu with quick reactions.
  const box = (await page.locator('.message-row').last().boundingBox())!
  await touch(page, 'touchStart', [{ x: box.x + 80, y: box.y + box.height / 2 }])
  await page.waitForTimeout(650)
  await touch(page, 'touchEnd', [])
  await expect(page.getByRole('menu')).toBeVisible()
  await expect(page.getByRole('menuitem', { name: 'React with 👍' })).toBeVisible()
  await page.keyboard.press('Escape')

  // Swipe in from the left edge: navigation drawer.
  await touch(page, 'touchStart', [{ x: 4, y: 400 }])
  for (const x of [60, 120, 180, 240]) await touch(page, 'touchMove', [{ x, y: 400 }])
  await touch(page, 'touchEnd', [])
  await expect(page.locator('.conversation-sidebar')).toBeVisible()
  await page.locator('.mobile-scrim').tap({ position: { x: 375, y: 400 } })

  await page.getByRole('button', { name: 'Choose an emoji' }).tap()
  await expect(page.locator('.expression-picker.is-sheet')).toBeVisible()
})

test('tablets keep the network rail and undock the channel list', async ({ browser }) => {
  const context = await browser.newContext({
    viewport: { width: 820, height: 1180 },
    hasTouch: true,
  })
  const page = await context.newPage()
  await signIn(page)
  await expect(page.locator('.network-rail')).toBeVisible()
  await expect(page.locator('.conversation-sidebar')).toBeHidden()
  await page.getByRole('button', { name: 'Open conversations' }).click()
  await expect(page.locator('.conversation-sidebar')).toBeVisible()
  await context.close()
})
