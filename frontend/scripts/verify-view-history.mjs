// Isolated demo only; no real account, IRC connection, or message is used.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium, webkit } from 'playwright'

const base = process.env.BASE ?? 'http://127.0.0.1:4179'
const engine = process.env.ENGINE === 'webkit' ? webkit : chromium
const browser = await engine.launch({ headless: true })
const context = await browser.newContext({
  viewport: process.env.MOBILE ? { width: 390, height: 844 } : { width: 1440, height: 900 },
  serviceWorkers: 'block',
})
const errors = []
let created = false
let stage = 'start'
let page
try {
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `history${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  created = true
  assert.equal((await (await context.request.get(`${base}/api/v1/bootstrap`)).json()).demo, true)
  // Provider requests use local responses and never reach the GIF service.
  await context.route('https://api.klipy.com/**', (route) =>
    route.fulfill({
      json: {
        data: route.request().url().includes('/categories')
          ? {
              categories: [
                { category: 'Hello', query: 'hello', preview_url: `${base}/media/weekend.jpg` },
              ],
            }
          : { data: [], has_next: false },
      },
    }),
  )
  // Only the navigation frame is exercised; administration data uses a disposable fixture.
  await context.route('**/api/v1/administration/**', (route) => {
    assert.equal(route.request().method(), 'GET')
    return route.fulfill({
      status: route.request().url().endsWith('/access') ? 200 : 503,
      json: route.request().url().endsWith('/access')
        ? { owner: true }
        : { message: 'Navigation fixture' },
    })
  })
  page = await context.newPage()
  page.setDefaultTimeout(6000)
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(base)
  await page.waitForURL(`${base}/channel/general`)
  const input = page.getByRole('textbox', { name: 'Message #general', exact: true })
  await input.fill('Draft stays in this conversation')
  const timeline = page.locator('.timeline-scroll')
  await timeline.evaluate((element) => {
    element.scrollTop = Math.max(0, element.scrollHeight - element.clientHeight - 80)
  })
  const scrollBefore = await timeline.evaluate((element) => element.scrollTop)
  const state = () =>
    page.evaluate(() => ({
      position: history.state.position,
      views: history.state.ircordViews,
      length: history.length,
    }))
  const settled = async () => {
    await page.waitForTimeout(120)
  }
  const back = async () => {
    await page.evaluate(() => history.back())
    await settled()
  }
  const forward = async () => {
    await page.evaluate(() => history.forward())
    await settled()
  }
  const close = () =>
    page
      .getByRole('button', { name: /^Close (dialog|viewer|media viewer)$/ })
      .last()
      .click()
  async function settings() {
    await page.getByRole('button', { name: 'User settings', exact: true }).first().click()
    await page.getByRole('dialog', { name: 'Settings', exact: true }).waitFor()
    await settled()
  }
  {
    const initial = await state()
    await settings()
    const appearance = await state()
    assert.equal(appearance.position, initial.position + 1, 'Opening settings is one entry')
    await page.getByRole('button', { name: 'Profile', exact: true }).click()
    await settled()
    await page.getByRole('button', { name: 'Notifications', exact: true }).click()
    await settled()
    assert.equal(
      (await state()).position,
      appearance.position + 2,
      'Each settings section adds one entry',
    )
    await back()
    await page.getByRole('heading', { name: 'Profile', exact: true }).waitFor()
    await back()
    await page.getByRole('heading', { name: 'Appearance', exact: true }).waitFor()
    await forward()
    await page.getByRole('heading', { name: 'Profile', exact: true }).waitFor()
    await forward()
    await page.getByRole('heading', { name: 'Notifications', exact: true }).waitFor()
    await page.screenshot({
      path: `../.data/history-259-${engine.name()}-${process.env.MOBILE ? 'mobile' : 'desktop'}.png`,
    })
    await close()
    await settled()
    assert.equal(
      (await state()).position,
      initial.position,
      'Explicit close consumes section entries',
    )
    assert.equal(await page.getByRole('dialog').count(), 0)
    assert.equal(await input.inputValue(), 'Draft stays in this conversation')
    assert(Math.abs((await timeline.evaluate((element) => element.scrollTop)) - scrollBefore) <= 1)
    await forward()
    await page.getByRole('dialog', { name: 'Settings', exact: true }).waitFor()
    await page.getByRole('heading', { name: 'Appearance', exact: true }).waitFor()
    await page.keyboard.press('Escape')
    await settled()
    assert.equal((await state()).position, initial.position)
    const length = (await state()).length
    for (let count = 0; count < 6; count++) {
      await settings()
      await page.keyboard.press('Escape')
      await settled()
      assert.equal((await state()).position, initial.position)
    }
    assert((await state()).length <= length, 'Repeated close/open keeps history bounded')
    await settings()
    await page.getByRole('button', { name: 'Account', exact: true }).click()
    const password = page.getByLabel('Current password', { exact: true })
    const secret = 'private-form-value-never-in-history'
    await password.fill(secret)
    await settled()
    const beforeTyping = (await state()).position
    await password.fill(`${secret}-edited`)
    await settled()
    assert.equal((await state()).position, beforeTyping, 'Typing does not add navigation')
    assert(!(await page.evaluate(() => JSON.stringify(history.state))).includes(secret))
    await page.getByRole('button', { name: 'Profile', exact: true }).click()
    await settled()
    await back()
    assert.equal(
      await password.inputValue(),
      `${secret}-edited`,
      'Cached forms keep unsaved input across Back',
    )
    await page.getByRole('button', { name: 'Delete account', exact: true }).click()
    await settled()
    assert.equal(await page.locator('[role="dialog"]').count(), 2)
    await back()
    assert.equal(await page.getByRole('dialog').count(), 1, 'Back dismisses only the confirmation')
    await forward()
    assert.equal(
      await page.getByRole('dialog').count(),
      1,
      'Forward skips a settled action confirmation',
    )
    await close()
    await settled()
    await page.getByRole('button', { name: 'Administration', exact: true }).click()
    await page.getByRole('dialog', { name: 'Administration', exact: true }).waitFor()
    await page.getByRole('button', { name: 'Database', exact: true }).click()
    await settled()
    await page.getByRole('button', { name: 'Users', exact: true }).click()
    await settled()
    await back()
    await page.getByRole('heading', { name: 'Database', exact: true }).waitFor()
    await back()
    await page.getByRole('heading', { name: 'Overview', exact: true }).waitFor()
    await forward()
    await page.getByRole('heading', { name: 'Database', exact: true }).waitFor()
    await close()
    await settled()
    stage = 'open profile'
    if (process.env.MOBILE) {
      await page.getByRole('button', { name: 'Members', exact: true }).click()
      await page.locator('.member-sidebar').getByRole('button', { name: /Mika/ }).click()
    } else {
      const profileButton = page
        .getByRole('button', { name: 'View Mika’s profile', exact: true })
        .first()
      await profileButton.scrollIntoViewIfNeeded()
      await settled()
      await profileButton.click()
    }
    await page.getByRole('dialog', { name: 'Mika', exact: true }).waitFor()
    await settled()
    await back()
    assert.equal(await page.getByRole('dialog', { name: 'Mika', exact: true }).count(), 0)
    stage = 'profile Forward'
    await forward()
    await page.getByRole('dialog', { name: 'Mika', exact: true }).waitFor()
    await close()
    await settled()
    if (process.env.MOBILE) {
      await page.keyboard.press('Escape')
      await settled()
    }
    await page.getByRole('button', { name: 'Open image', exact: true }).first().click()
    await page.locator('.media-viewer').waitFor()
    await settled()
    await back()
    assert.equal(await page.locator('.media-viewer').count(), 0)
    await forward()
    await page.locator('.media-viewer').waitFor()
    await close()
    await settled()
    assert.equal(await input.inputValue(), 'Draft stays in this conversation')
    stage = 'reaction picker categories'
    const firstMessage = page.locator('.message-row').first()
    await firstMessage.hover()
    await firstMessage.getByRole('button', { name: 'Add reaction', exact: true }).click()
    const reaction = page.getByRole('dialog', { name: 'Choose a reaction', exact: true })
    await reaction.waitFor()
    const animals = reaction.getByRole('button', { name: 'Animals & nature', exact: true })
    await animals.click()
    await settled()
    await back()
    assert.equal(await animals.getAttribute('aria-pressed'), 'false')
    await forward()
    assert.equal(await animals.getAttribute('aria-pressed'), 'true')
    await page.keyboard.press('Escape')
    await settled()
    assert.equal(await page.locator('.expression-picker').count(), 0)
    stage = 'GIF destinations'
    await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
    const picker = page.locator('.expression-picker')
    await picker.getByRole('button', { name: 'Favorites', exact: true }).click()
    await settled()
    await back()
    await picker.getByRole('button', { name: 'Trending', exact: true }).waitFor()
    await forward()
    await picker.getByRole('textbox', { name: 'Search favorite GIFs', exact: true }).waitFor()
    await back()
    await picker.getByRole('button', { name: 'Hello', exact: true }).click()
    await settled()
    await back()
    await picker.getByRole('button', { name: 'Hello', exact: true }).waitFor()
    await forward()
    assert.equal(
      await picker.getByRole('textbox', { name: 'Search Klipy', exact: true }).inputValue(),
      'hello',
    )
    await page.keyboard.press('Escape')
    await settled()
    assert.equal(await page.locator('.expression-picker').count(), 0)
    // A profile's edit action changes the navigational view in the same modal layer.
    await page.getByRole('button', { name: /open your profile/i }).click()
    await page.getByRole('button', { name: 'Edit profile', exact: true }).click()
    await page.getByRole('heading', { name: 'Profile', exact: true }).waitFor()
    await settled()
    await back()
    await page.getByRole('dialog', { name: 'Karbowiak', exact: true }).waitFor()
    await forward()
    await page.getByRole('heading', { name: 'Profile', exact: true }).waitFor()
    await close()
    await settled()
    stage = 'channel sections'
    await page.locator('.channel-name-button').click()
    const channelDialog = page.getByRole('dialog', { name: '#general settings', exact: true })
    await channelDialog.waitFor()
    await channelDialog.getByRole('button', { name: 'Modes', exact: true }).click()
    await settled()
    await channelDialog.getByRole('button', { name: 'Registration', exact: true }).click()
    await settled()
    await back()
    assert.equal(
      await channelDialog
        .getByRole('button', { name: 'Modes', exact: true })
        .getAttribute('aria-current'),
      'page',
    )
    await forward()
    assert.equal(
      await channelDialog
        .getByRole('button', { name: 'Registration', exact: true })
        .getAttribute('aria-current'),
      'page',
    )
    await close()
    await settled()
    if (process.env.MOBILE) {
      const navigation = page.getByRole('button', { name: 'Open conversations', exact: true })
      await navigation.click()
      await settled()
      assert.equal(await page.locator('.workspace.mobile-navigation-open').count(), 1)
      await back()
      assert.equal(await page.locator('.workspace.mobile-navigation-open').count(), 0)
      await forward()
      assert.equal(await page.locator('.workspace.mobile-navigation-open').count(), 1)
      await page.getByRole('button', { name: 'Close conversations', exact: true }).last().click()
      await settled()
      await page.getByRole('button', { name: 'Members', exact: true }).click()
      await settled()
      await back()
      assert.equal(await page.locator('.workspace.mobile-members-open').count(), 0)
      await forward()
      assert.equal(await page.locator('.workspace.mobile-members-open').count(), 1)
      await page.keyboard.press('Escape')
      await settled()
      assert.equal(await page.locator('.workspace.mobile-members-open').count(), 0)
      await navigation.click()
      await settled()
    }
    await page
      .locator('.conversation-sidebar')
      .getByRole('button', { name: /^Search/ })
      .first()
      .click()
    await page.waitForURL(`${base}/search`)
    await back()
    await page.waitForURL(`${base}/channel/general`)
    assert.equal(await input.inputValue(), 'Draft stays in this conversation')
    await forward()
    await page.waitForURL(`${base}/search`)
    stage = 'search sections and filters'
    await page.getByRole('combobox', { name: 'Search workspace', exact: true }).press('Escape')
    await page.getByRole('tab', { name: 'Messages', exact: true }).click()
    await settled()
    await back()
    assert.equal(
      await page
        .getByRole('tab', { name: 'Conversations', exact: true })
        .getAttribute('aria-selected'),
      'true',
    )
    await forward()
    assert.equal(
      await page.getByRole('tab', { name: 'Messages', exact: true }).getAttribute('aria-selected'),
      'true',
    )
    await page.getByRole('button', { name: /^Filters/ }).click()
    await page.getByRole('dialog', { name: 'Filters', exact: true }).waitFor()
    await settled()
    await back()
    assert.equal(await page.getByRole('dialog').count(), 0)
    await forward()
    await page.getByRole('dialog', { name: 'Filters', exact: true }).waitFor()
    await close()
    await settled()
    stage = 'reload available view'
    await settings()
    await page.getByRole('button', { name: 'Notifications', exact: true }).click()
    await settled()
    const beforeReload = (await state()).position
    await page.reload()
    await page.getByRole('heading', { name: 'Notifications', exact: true }).waitFor()
    assert.equal(
      (await state()).position,
      beforeReload,
      'Reload restores available views without duplicate entries',
    )
    await close()
    await settled()
    assert.equal(new URL(page.url()).pathname, '/search')
  }
  assert.deepEqual(errors, [])
  console.log(
    `View history passes in ${engine.name()}, ${process.env.MOBILE ? '390×844 viewport' : '1440×900 viewport'}.`,
  )
} catch (error) {
  console.error({
    stage,
    url: page?.url(),
    headings: await page?.getByRole('heading').allTextContents(),
    position: await page?.evaluate(() => history.state?.position),
  })
  await page?.screenshot({
    path: `../.data/history-259-failure-${engine.name()}-${process.env.MOBILE ? 'mobile' : 'desktop'}.png`,
  })
  throw error
} finally {
  if (created) await context.request.delete(`${base}/api/v1/auth/account`)
  await browser.close()
}
