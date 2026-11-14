// Run against an isolated demo server, never the live backend.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium } from 'playwright'

const base = process.env.DEMO_URL || 'http://127.0.0.1:4173'
const browser = await chromium.launch({ headless: true })
try {
  // Stop the application bundle to inspect the actual pre-hydration document.
  for (const [system, saved, expected] of [
    ['light', null, 'light'],
    ['dark', null, 'dark'],
    ['light', 'dark', 'dark'],
    ['dark', 'light', 'light'],
    ['light', 'system', 'light'],
  ]) {
    const context = await browser.newContext({ colorScheme: system, serviceWorkers: 'block' })
    if (saved) {
      await context.addInitScript((mode) => {
        localStorage.setItem(
          'ircord.appearance.compiled',
          JSON.stringify({
            mode,
            dark: { attrs: { 'data-theme': 'dark' }, css: ':root:root { --main: #111111; }' },
            light: { attrs: { 'data-theme': 'light' }, css: ':root:root { --main: #eeeeee; }' },
          }),
        )
      }, saved)
    }
    await context.route('**/*', (route) =>
      route.request().resourceType() === 'script' ? route.abort() : route.continue(),
    )
    const page = await context.newPage()
    await page.goto(base, { waitUntil: 'domcontentloaded' })
    assert.equal(await page.locator('html').getAttribute('data-theme'), expected)
    assert.equal(await page.evaluate(() => document.documentElement.style.colorScheme), expected)
    await context.close()
  }

  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    serviceWorkers: 'block',
    reducedMotion: 'no-preference',
  })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `motion${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  assert.equal((await (await context.request.get(`${base}/api/v1/bootstrap`)).json()).demo, true)
  const page = await context.newPage()
  await page.goto(base)
  await page.waitForURL('**/channel/general')
  assert.equal(
    await page.evaluate(() => JSON.parse(localStorage.getItem('ircord.appearance')).settings.mode),
    'system',
  )
  await page.emulateMedia({ colorScheme: 'light' })
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'light')
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.waitForFunction(() => document.documentElement.dataset.theme === 'dark')
  // Measure the chat edge through the toggle: a sliding panel alone can conceal a layout snap.
  async function toggleTrack(trigger, target, edge) {
    const frames = await page.evaluate(
      async ({ trigger, target, edge }) => {
        const read = () => document.querySelector(target).getBoundingClientRect()[edge]
        const values = [read()]
        document.querySelector(trigger).click()
        const start = performance.now()
        while (performance.now() - start < 280) {
          await new Promise(requestAnimationFrame)
          values.push(read())
        }
        return values
      },
      { trigger, target, edge },
    )
    const start = frames[0]
    const end = frames.at(-1)
    assert.ok(Math.abs(end - start) > 100, 'The toggle changed the chat layout')
    const low = Math.min(start, end)
    const high = Math.max(start, end)
    assert.ok(
      frames.some((value) => value > low + 10 && value < high - 10),
      `${trigger} should move the chat edge through intermediate positions, not snap`,
    )
    assert.ok(
      frames.every((value) => value >= low - 1 && value <= high + 1),
      'No layout overshoot',
    )
  }
  async function sidebarTrack(trigger, expectOutDuringCover) {
    const frames = await page.evaluate(
      async ({ trigger, expectOutDuringCover }) => {
        const read = () => {
          const workspace = document.querySelector('.workspace')
          const expanded = document.querySelector('.sidebar-content')
          const compact = document.querySelector('.sidebar-compact')
          return {
            left: document.querySelector('.chat-main').getBoundingClientRect().left,
            covering: workspace.classList.contains('sidebar-covering'),
            revealing: workspace.classList.contains('sidebar-revealing'),
            expandedVisible:
              !!expanded &&
              getComputedStyle(expanded).display !== 'none' &&
              expanded.offsetWidth > 0,
            compactVisible:
              !!compact && getComputedStyle(compact).display !== 'none' && compact.offsetWidth > 0,
            expectOutDuringCover,
          }
        }
        const values = [read()]
        document.querySelector(trigger).click()
        const start = performance.now()
        while (performance.now() - start < 680) {
          await new Promise(requestAnimationFrame)
          values.push(read())
        }
        return values
      },
      { trigger, expectOutDuringCover },
    )
    const start = frames[0].left
    const end = frames.at(-1).left
    assert.ok(Math.abs(end - start) > 100, `${trigger} changed the chat layout`)
    const minLeft = Math.min(...frames.map((frame) => frame.left))
    const minIndex = frames.findIndex((frame) => frame.left === minLeft)
    assert.ok(minLeft <= 4, `${trigger} expands chat to the left edge (min ${minLeft})`)
    assert.ok(
      minIndex > 0 && minIndex < frames.length - 1,
      `${trigger} rebounds after hitting left edge`,
    )
    const coverFrames = frames.filter((frame) => frame.covering)
    assert.ok(coverFrames.length > 0, `${trigger} enters cover phase`)
    for (const frame of coverFrames) {
      if (expectOutDuringCover === 'expanded') {
        assert.equal(frame.expandedVisible, true, 'Expanded sidebar stays visible during cover')
      } else {
        assert.equal(frame.compactVisible, true, 'Mini sidebar stays visible during cover')
      }
    }
    const maxBound = Math.max(start, end)
    assert.ok(
      frames.every((frame) => frame.left >= -1 && frame.left <= maxBound + 1),
      'No layout overshoot past expanded sidebar edge',
    )
  }
  const content = page.locator('.sidebar-content')
  await sidebarTrack('.sidebar-collapse', 'expanded')
  assert.equal(await content.isVisible(), false)
  await sidebarTrack('.sidebar-restore', 'compact')
  assert.equal(await content.isVisible(), true)
  const timeline = page.locator('.timeline-scroll')
  await timeline.evaluate((el) => (el.scrollTop = 200))
  const scrollBefore = await timeline.evaluate((el) => el.scrollTop)
  await toggleTrack('.member-toggle', '.chat-content', 'right')
  assert.equal(await page.locator('.member-sidebar').count(), 0)
  assert.equal(await timeline.evaluate((el) => el.scrollTop), scrollBefore)
  await toggleTrack('.member-toggle', '.chat-content', 'right')
  await page.locator('.member-sidebar').waitFor()
  await page.locator('.conversation-search-toggle').click()
  await page.locator('.conversation-search').waitFor()
  await page.locator('.member-toggle').click()
  await page.locator('.member-sidebar:not(.conversation-search)').waitFor()
  // Pointer-driven focus restoration must not leave labels pinned over the workspace.
  const settings = page.getByRole('button', { name: 'User settings', exact: true })
  const tooltip = page.locator('[data-slot="tooltip-content"]')
  await settings.click()
  await page.getByRole('dialog').waitFor()
  await page.mouse.click(12, 450)
  await page.getByRole('dialog').waitFor({ state: 'hidden' })
  await page.waitForTimeout(600)
  assert.equal(
    await tooltip.count(),
    0,
    'Closing settings with the pointer must not reopen its label',
  )
  assert.equal(
    await settings.evaluate((el) => el === document.activeElement),
    true,
    'Focus returns to settings',
  )
  await page.locator('.sidebar-collapse').click()
  await page.waitForTimeout(650)
  assert.equal(await tooltip.count(), 0, 'Collapsing the sidebar must not pin the restore label')
  await page.locator('.sidebar-restore').click()
  await page.waitForTimeout(650)
  assert.equal(await tooltip.count(), 0, 'Expanding the sidebar must not pin a label')
  await settings.hover()
  await tooltip.waitFor()
  await page.mouse.move(700, 80)
  await tooltip.waitFor({ state: 'hidden' })
  await page.keyboard.press('Tab')
  await settings.focus()
  await tooltip.waitFor()
  await page.keyboard.press('Escape')
  await tooltip.waitFor({ state: 'hidden' })
  const handle = page.locator('.conversation-sidebar > .resize-handle')
  const beforeWidth = await page.locator('.conversation-sidebar').evaluate((el) => el.clientWidth)
  await handle.focus()
  await page.keyboard.press('ArrowRight')
  const afterWidth = await page.locator('.conversation-sidebar').evaluate((el) => el.clientWidth)
  assert.equal(afterWidth - beforeWidth, 16, 'Keyboard resizing takes effect immediately')
  await page.setViewportSize({ width: 390, height: 844 })
  await page.locator('.mobile-menu').click()
  assert.equal(await page.locator('.conversation-sidebar').getAttribute('role'), 'dialog')
  assert.equal(await page.locator('.chat-main').getAttribute('inert'), '')
  assert.match(
    await page
      .locator('.conversation-sidebar')
      .evaluate((el) => getComputedStyle(el).transitionProperty),
    /transform/,
    'Mobile navigation uses a reversible transform transition',
  )
  // Reverse before the entrance finishes; the same drawer must settle closed.
  await page.keyboard.press('Escape')
  await page.waitForTimeout(250)
  assert.equal(await page.locator('.conversation-sidebar').isVisible(), false)
  await page.locator('.member-toggle').click()
  assert.equal(await page.locator('.member-sidebar').getAttribute('role'), 'dialog')
  await page.keyboard.press('Escape')
  await page.waitForTimeout(250)
  assert.equal(await page.locator('.member-sidebar').isVisible(), false)
  await page.locator('.conversation-search-toggle').click()
  assert.equal(await page.locator('.conversation-search').getAttribute('role'), 'dialog')
  await page.keyboard.press('Escape')
  // The first Escape dismisses the search filter suggestions when they are open.
  if (await page.locator('.conversation-search[role="dialog"]').isVisible())
    await page.keyboard.press('Escape')
  await page.waitForTimeout(250)
  assert.equal(await page.locator('.conversation-search').isVisible(), false)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.locator('.mobile-menu').click()
  assert.equal(
    await page
      .locator('.conversation-sidebar')
      .evaluate((el) => getComputedStyle(el).transitionDuration),
    '0s',
    'OS reduced motion suppresses panel movement',
  )
  await page.keyboard.press('Escape')
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  await page.evaluate(() => {
    const local = JSON.parse(localStorage.getItem('ircord.appearance'))
    local.settings.motion = 'reduced'
    local.sync = false
    localStorage.setItem('ircord.appearance', JSON.stringify(local))
  })
  await page.reload()
  await page.waitForFunction(() => document.documentElement.dataset.motion === 'reduced')
  await page.locator('.mobile-menu').click()
  assert.equal(
    await page
      .locator('.conversation-sidebar')
      .evaluate((el) => getComputedStyle(el).transitionDuration),
    '0s',
    'App reduced motion suppresses panel movement',
  )
  console.log('Appearance, desktop/mobile panel layout, and tooltip interaction checks pass.')
  await context.close()
} finally {
  await browser.close()
}
