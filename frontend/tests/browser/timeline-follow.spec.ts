import { deflateSync } from 'node:zlib'
import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

/** A plain grey PNG, large enough to change the layout when it finishes loading. */
function png(width: number, height: number) {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    return c >>> 0
  })
  const crc = (buf: Buffer) => {
    let c = 0xffffffff
    for (const b of buf) c = crcTable[(c ^ b) & 0xff]! ^ (c >>> 8)
    return (c ^ 0xffffffff) >>> 0
  }
  const chunk = (type: string, data: Buffer) => {
    const body = Buffer.concat([Buffer.from(type), data])
    const out = Buffer.alloc(body.length + 8)
    out.writeUInt32BE(data.length, 0)
    body.copy(out, 4)
    out.writeUInt32BE(crc(body), body.length + 4)
    return out
  }
  const header = Buffer.alloc(13)
  header.writeUInt32BE(width, 0)
  header.writeUInt32BE(height, 4)
  header.set([8, 0, 0, 0, 0], 8) // 8-bit greyscale
  const row = Buffer.concat([Buffer.from([0]), Buffer.alloc(width, 0x88)])
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', header),
    chunk('IDAT', deflateSync(Buffer.concat(Array.from({ length: height }, () => row)))),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

// The timeline follows the conversation while you're at the latest message (#138), and stops
// only when you scroll up far enough to see the “new messages” control.

const timeline = (page: Page) => page.locator('.timeline-scroll')
const distance = (page: Page) =>
  timeline(page).evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight)
const pill = (page: Page) => page.locator('.jump-latest')

async function openBusy(page: Page, rate: number, seconds: number) {
  const load = await page.request.post('/api/v1/_demo/busy', {
    data: { members: 40, history: 120, rate, churn: 4, seconds },
  })
  expect(load.ok()).toBe(true)
  const drawer = page.getByRole('button', { name: 'Open conversations' })
  if (await drawer.isVisible()) await drawer.click()
  await page.locator('.conversation-link').filter({ hasText: /^busy/ }).click()
  await expect(page.getByRole('combobox', { name: 'Message #busy' })).toBeVisible()
  await expect.poll(() => distance(page)).toBeLessThan(2)
}

/** Samples the distance from the bottom for `ms`, returning the largest one seen. */
async function worstDistance(page: Page, ms: number) {
  let worst = 0
  for (const end = Date.now() + ms; Date.now() < end;) {
    worst = Math.max(worst, await distance(page))
    await page.waitForTimeout(100)
  }
  return worst
}

for (const [name, size] of [
  ['desktop', { width: 1440, height: 900 }],
  ['phone', { width: 390, height: 844 }],
] as const)
  test(`stays at the latest message while a busy channel floods in (${name})`, async ({ page }) => {
    await page.setViewportSize(size)
    await signIn(page)
    await openBusy(page, 40, 8)
    // Rapid arrivals, wrapped lines and collapsing chatter never leave the view behind.
    expect(await worstDistance(page, 4000)).toBeLessThan(80)
    await expect(pill(page)).toBeHidden()
    await expect.poll(() => distance(page)).toBeLessThan(2)
  })

test('scrolling up stops following until you jump back to the latest', async ({ page }) => {
  await signIn(page)
  await openBusy(page, 20, 12)
  await timeline(page).hover()
  await page.mouse.wheel(0, -900)
  await expect(pill(page)).toBeVisible()
  // What the reader is looking at: the row in the middle of the timeline. (scrollTop itself moves
  // when older history loads above.)
  const reading = () =>
    timeline(page).evaluate((el) => {
      const box = el.getBoundingClientRect()
      const row = document
        .elementFromPoint(box.left + box.width / 2, box.top + box.height / 2)
        ?.closest('.virtual-row')
      return row ? `${row.textContent}`.slice(0, 120) : ''
    })
  await page.waitForTimeout(300)
  const before = await reading()
  await page.waitForTimeout(1500)
  // New messages arrived below, but the reader wasn't pulled away from what they were reading.
  expect(await reading()).toBe(before)
  await expect(pill(page)).toContainText('new message')

  await pill(page).click()
  await expect(pill(page)).toBeHidden()
  expect(await worstDistance(page, 2500)).toBeLessThan(80)
  await expect.poll(() => distance(page)).toBeLessThan(2)
})

test('keeps following when media below finishes loading and grows the timeline', async ({
  page,
}) => {
  // The image answers late, after the message has been laid out with its placeholder size.
  await page.route('https://i.imgur.com/**', async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 1500))
    await route.fulfill({ contentType: 'image/png', body: png(600, 900) })
  })
  await signIn(page)
  const composer = page.getByRole('combobox', { name: 'Message #general' })
  for (let i = 0; i < 12; i++) {
    await composer.fill(`filler ${i}`)
    await composer.press('Enter')
  }
  await expect.poll(() => distance(page)).toBeLessThan(2)
  await composer.fill('https://i.imgur.com/tall.png')
  await composer.press('Enter')
  // The echo bot repeats it half a second later, as an incoming message; both images load late.
  await expect(
    page.locator('.message-row').filter({ hasText: 'i.imgur.com/tall.png' }),
  ).toHaveCount(2)
  await page.waitForTimeout(2500)
  await expect.poll(() => distance(page)).toBeLessThan(2)
  await expect(pill(page)).toBeHidden()
})

test('keeps the latest message in view when the keyboard or composer takes space', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  const composer = page.getByRole('combobox', { name: 'Message #general' })
  for (let i = 0; i < 15; i++) {
    await composer.fill(`filler ${i}`)
    await composer.press('Enter')
  }
  await expect.poll(() => distance(page)).toBeLessThan(2)
  // No message arrives here: the timeline itself gets shorter.
  await composer.fill('line one')
  for (let i = 0; i < 4; i++) {
    await composer.press('Shift+Enter')
    await composer.pressSequentially('more text')
  }
  await expect.poll(() => distance(page)).toBeLessThan(2)
  // A soft keyboard shrinks the viewport.
  await page.setViewportSize({ width: 390, height: 500 })
  await expect.poll(() => distance(page)).toBeLessThan(2)
  // Touching the timeline afterwards doesn't count as scrolling away.
  await timeline(page).dispatchEvent('scroll')
  await expect(pill(page)).toBeHidden()
  await composer.fill('')
  await page.setViewportSize({ width: 390, height: 844 })
  await expect.poll(() => distance(page)).toBeLessThan(2)
  // And the next message still follows.
  await composer.fill('after the keyboard')
  await composer.press('Enter')
  await expect(page.locator('.message-row').filter({ hasText: 'after the keyboard' })).toHaveCount(
    2,
  )
  await expect.poll(() => distance(page)).toBeLessThan(2)
  await expect(pill(page)).toBeHidden()
})

test('older history loading above keeps the row you are reading in place', async ({ page }) => {
  await signIn(page)
  // A quiet channel: nothing new arrives, only older pages load as you near the top.
  await openBusy(page, 1, 1)
  await page.waitForTimeout(1500)
  await timeline(page).hover()
  const firstRow = () =>
    timeline(page).evaluate((el) => {
      const top = el.getBoundingClientRect().top
      const rows = [...el.querySelectorAll<HTMLElement>('.virtual-row')]
        .filter((row) => row.getBoundingClientRect().top >= top)
        .sort((a, b) => a.getBoundingClientRect().top - b.getBoundingClientRect().top)
      return rows[0] ? `${rows[0].dataset.key}@${Math.round(rows[0].getBoundingClientRect().top - top)}` : ''
    })
  for (let i = 0; i < 12; i++) {
    await page.mouse.wheel(0, -1200)
    await page.waitForTimeout(150)
  }
  // Older pages arrive while the reader sits near the top; what they see doesn't move.
  await page.waitForTimeout(400)
  const reading = await firstRow()
  await page.waitForTimeout(1500)
  expect(await firstRow()).toBe(reading)
  await expect(pill(page)).toBeVisible()
})

test('the new-messages count leaves out older history loaded while reading back', async ({
  page,
}) => {
  await signIn(page)
  // A quiet channel with deep history: the short flood is over before the reader scrolls up.
  await openBusy(page, 1, 1)
  await page.waitForTimeout(1500)
  await timeline(page).hover()
  const older = page.waitForRequest((r) => r.url().includes('before='))
  // Read back far enough for older pages to load above the view.
  for (let i = 0; i < 12; i++) {
    await page.mouse.wheel(0, -3000)
    await page.waitForTimeout(150)
  }
  await older
  await expect(pill(page)).toBeVisible()
  await page.waitForTimeout(500)
  // Nothing arrived after the reader scrolled up, so nothing is counted as new.
  await expect(pill(page)).toHaveText(/^Jump to present/)
})
