// Run against an isolated demo server, never the live backend.
// Copy, open and download stay off an embed until it is hovered, focused or tapped.
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium, devices } from 'playwright'

const base = process.env.DEMO_URL || 'http://127.0.0.1:4173'
const browser = await chromium.launch({ headless: true })

async function open(options) {
  const context = await browser.newContext({ serviceWorkers: 'block', ...options })
  const signup = await context.request.post(`${base}/api/v1/auth/signup`, {
    data: { username: `embed${Date.now().toString(36)}`, password: randomUUID() },
  })
  assert.equal(signup.status(), 201)
  assert.equal((await (await context.request.get(`${base}/api/v1/bootstrap`)).json()).demo, true)
  const page = await context.newPage()
  await page.goto(base)
  await page.waitForURL('**/channel/general')
  // The seeded road-trip photo in #general.
  const image = page.locator('img[alt^="A quiet road"]').first()
  const embed = page.locator('.message-link').filter({ has: image }).first()
  const actions = embed.locator('.media-link-actions')
  await actions.waitFor({ state: 'attached' })
  await embed.scrollIntoViewIfNeeded()
  // Copy and open, and the image toolbar's own download button.
  const controls = [actions, embed.getByRole('link', { name: 'Download file', exact: true })]
  return { context, page, image, embed, actions, controls }
}

/** Painted opacity, including fading ancestors, and whether a tap at its centre reaches it. */
const state = (locator) =>
  locator.evaluate((el) => {
    let opacity = 1
    for (let node = el; node; node = node.parentElement)
      opacity *= Number(getComputedStyle(node).opacity)
    const box = el.getBoundingClientRect()
    const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2)
    return { opacity, reachable: !!hit && el.contains(hit) }
  })
async function shown(controls) {
  for (const control of controls) {
    const { opacity, reachable } = await state(control)
    if (!(opacity > 0.99 && reachable)) return false
  }
  return true
}
async function hidden(controls) {
  for (const control of controls) {
    const { opacity, reachable } = await state(control)
    if (!(opacity < 0.01 && !reachable)) return false
  }
  return true
}
const settle = (page) => page.waitForTimeout(400)

try {
  // Mouse: hidden until hover, and hidden again once the pointer leaves.
  {
    const label = 'mouse'
    const { context, page, image, actions, controls } = await open({
      viewport: { width: 1280, height: 900 },
    })
    await page.mouse.move(2, 2)
    await settle(page)
    assert.ok(await hidden(controls), `${label}: buttons stay off the embed until hover`)
    await image.hover()
    await settle(page)
    assert.ok(await shown(controls), `${label}: hovering the embed shows its buttons`)
    await page.mouse.move(2, 2)
    await settle(page)
    assert.ok(await hidden(controls), `${label}: buttons hide when the pointer leaves`)
    // Keyboard: focusing a hidden button reveals the group.
    await actions.getByRole('button').first().focus()
    await settle(page)
    assert.ok(await shown(controls), `${label}: keyboard focus shows the buttons`)
    await context.close()
  }

  // Touch screen without hover: hidden until the embed is tapped, hidden after a tap elsewhere.
  {
    const { context, page, image, actions, controls } = await open({ ...devices['Pixel 7'] })
    // Both media queries used to force the buttons on; neither may do so now.
    assert.ok(
      await page.evaluate(
        () => matchMedia('(hover: none)').matches && matchMedia('(any-pointer: coarse)').matches,
      ),
      'The phone profile has no hover and a coarse pointer',
    )
    await settle(page)
    assert.ok(await hidden(controls), 'touch: buttons stay off the embed until it is tapped')
    await image.tap()
    // The tap also opens the viewer; close it to check the inline buttons.
    const viewer = page.getByRole('dialog')
    await viewer.waitFor()
    await page.keyboard.press('Escape')
    await viewer.waitFor({ state: 'hidden' })
    // Closing the viewer returns focus into the embed, which would reveal the buttons by
    // itself. Drop it so only the tap can keep them shown.
    await page.evaluate(() => document.activeElement?.blur())
    await settle(page)
    assert.equal(
      await image.evaluate((el) => el.closest('.message-link').contains(document.activeElement)),
      false,
      'touch: nothing in the embed has focus',
    )
    assert.ok(await shown(controls), 'touch: tapping the embed shows its buttons')
    await actions.getByRole('link').first().waitFor()
    await page.locator('.message-text').first().tap()
    await settle(page)
    assert.ok(await hidden(controls), 'touch: a tap elsewhere hides the buttons again')
    await context.close()
  }
  console.log('embed actions: ok')
} finally {
  await browser.close()
}
