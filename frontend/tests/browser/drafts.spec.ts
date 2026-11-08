import { expect, test, type Page } from '@playwright/test'
import { signIn } from './helpers'

const draftsEntry = (page: Page) =>
  page.getByRole('complementary', { name: 'Conversations' }).locator('.drafts-link')
const dialog = (page: Page) => page.getByRole('dialog', { name: 'Drafts' })
const rows = (page: Page) => dialog(page).locator('.draft-row')
/** This account's stored drafts and edit times, as the browser keeps them. */
const stored = (page: Page) =>
  page.evaluate(() => {
    const key = Object.keys(localStorage).find((k) => k.endsWith('.drafts'))!
    return {
      key,
      drafts: JSON.parse(localStorage.getItem(key) ?? '{}') as Record<string, string>,
      times: JSON.parse(
        localStorage.getItem(`${key.slice(0, -'.drafts'.length)}.draft-times`) ?? '{}',
      ),
    }
  })

test('lists drafts across networks, resumes them without sending and discards with undo', async ({
  page,
}) => {
  await signIn(page)
  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, none on this device')

  // A draft saved before edit times existed: only the plain text payload.
  const general = page.getByRole('combobox', { name: 'Message #general' })
  await page.locator('.conversation-link').filter({ hasText: 'development' }).click()
  const development = page.getByRole('combobox', { name: 'Message #development' })
  await development.fill('legacy words')
  await expect
    .poll(async () => Object.values((await stored(page)).drafts))
    .toContain('legacy words')
  const { key } = await stored(page)
  await page.evaluate((k) => {
    // After the app saves its drafts on pagehide (registered earlier, so it runs first).
    window.addEventListener('pagehide', () => {
      localStorage.removeItem(`${k.slice(0, -'.drafts'.length)}.draft-times`)
      // Also a draft whose conversation is gone from this workspace.
      const drafts = JSON.parse(localStorage.getItem(k)!)
      drafts['conversation-that-left'] = 'stranded line one\nstranded line two'
      localStorage.setItem(k, JSON.stringify(drafts))
    })
  }, key)
  await page.reload()
  await expect(development).toHaveValue('legacy words')

  // Same-named channel on a second network.
  await page.getByRole('button', { name: 'Add a network', exact: true }).click()
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill('My place')
  await page.getByRole('textbox', { name: 'Hostname' }).fill('irc.example.org')
  await page.getByRole('button', { name: 'Add network', exact: true }).click()
  await expect(page.locator('.network-heading')).toContainText('My place')
  await general.fill('over at my place')

  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, 3 on this device')
  await draftsEntry(page).click()
  await expect(dialog(page)).toContainText('on this device only')
  // Newest edit first; the legacy draft says it has no known time; the stranded one is kept.
  await expect(rows(page).nth(0)).toContainText('#general')
  await expect(rows(page).nth(0)).toContainText('My place')
  await expect(rows(page).nth(0)).toContainText('over at my place')
  const legacy = rows(page).filter({ hasText: 'legacy words' })
  await expect(legacy).toContainText('#development')
  await expect(legacy).toContainText('Edited earlier')
  const stranded = rows(page).filter({ hasText: 'stranded line one stranded line two' })
  await expect(stranded).toContainText('no longer in your workspace')
  await expect(stranded.locator('.draft-open')).toBeDisabled()
  await stranded.getByRole('button', { name: /^Show full draft/ }).click()
  await expect(stranded.getByRole('textbox', { name: /^Full draft/ })).toHaveValue(
    'stranded line one\nstranded line two',
  )

  // Opening resumes the other network's conversation with the caret after the text; nothing sent.
  // The row's name carries its place and preview for screen readers.
  await expect(legacy.locator('.draft-open')).toHaveAccessibleName(/#development.*legacy words/)
  await legacy.locator('.draft-open').focus()
  await page.keyboard.press('Enter')
  await expect(dialog(page)).toHaveCount(0)
  await expect(page.locator('.network-heading')).not.toContainText('My place')
  await expect(development).toBeFocused()
  await expect(development).toHaveValue('legacy words')
  expect(await development.evaluate((el: HTMLTextAreaElement) => el.selectionStart)).toBe(12)
  await page.keyboard.type('!')
  await expect(development).toHaveValue('legacy words!')
  await expect(page.locator('.message-text').filter({ hasText: 'legacy words' })).toHaveCount(0)

  // Sending removes it from the list; the other network's draft is untouched.
  await development.press('Enter')
  await expect(page.locator('.message-text').filter({ hasText: /^legacy words!$/ })).toHaveCount(2)
  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, 2 on this device')

  // Discard with Undo, then discard for good.
  await draftsEntry(page).click()
  const place = rows(page).filter({ hasText: 'over at my place' })
  await place.getByRole('button', { name: 'Discard draft in #general on My place' }).click()
  await expect(place).toHaveCount(0)
  await dialog(page).getByRole('button', { name: 'Undo' }).click()
  await expect(rows(page).filter({ hasText: 'over at my place' })).toHaveCount(1)
  await stranded.getByRole('button', { name: /Discard draft/ }).click()
  await expect(rows(page)).toHaveCount(1)
  await page.keyboard.press('Escape')
  await expect(dialog(page)).toHaveCount(0)

  // Kept across reloads, with the edit time stored beside the unchanged text payload.
  await page.reload()
  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, 1 on this device')
  const after = await stored(page)
  expect(Object.values(after.drafts)).toEqual(['over at my place'])
  expect(Object.keys(after.times)).toEqual(Object.keys(after.drafts))
})

test('drafts from a previous account on this browser are not shown', async ({ page, context }) => {
  await signIn(page)
  await page.getByRole('combobox', { name: 'Message #general' }).fill('first account only')
  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, 1 on this device')
  await page.waitForTimeout(400)
  await context.clearCookies()
  await signIn(page)
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toHaveValue('')
  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, none on this device')
})

test('the drafts list works on a phone-sized screen', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('typed on the go')
  await page.getByRole('button', { name: 'Open conversations' }).click()
  await draftsEntry(page).click()
  const row = rows(page).filter({ hasText: 'typed on the go' })
  await expect(row).toBeVisible()
  const width = await dialog(page).evaluate((el) => el.scrollWidth <= el.clientWidth)
  expect(width).toBe(true)
  await row.locator('.draft-open').click()
  await expect(input).toBeFocused()
  await expect(input).toHaveValue('typed on the go')
})

test("opening the open conversation's draft puts the caret after its text", async ({ page }) => {
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('resume from the end')
  await input.evaluate((el: HTMLTextAreaElement) => el.setSelectionRange(0, 0))
  // The entry is there from the start, so the first character never moves the list.
  const before = await page.locator('.sidebar-section').first().boundingBox()
  await draftsEntry(page).click()
  const row = rows(page).filter({ hasText: 'resume from the end' })
  await expect(row).toContainText('Open now')
  await row.locator('.draft-open').click()
  await expect(dialog(page)).toHaveCount(0)
  await expect(input).toBeFocused()
  expect(await input.evaluate((el: HTMLTextAreaElement) => el.selectionStart)).toBe(19)
  await input.press('Enter')
  await expect(draftsEntry(page)).toHaveAccessibleName('Drafts, none on this device')
  expect(await page.locator('.sidebar-section').first().boundingBox()).toEqual(before)
})

test('drafts saved in another tab show up, and a stale copy cannot discard them', async ({
  page,
  context,
}) => {
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('older text from tab A')
  await expect
    .poll(async () => Object.values((await stored(page)).drafts))
    .toContain('older text from tab A')
  await draftsEntry(page).click()
  await expect(rows(page).filter({ hasText: 'older text from tab A' })).toHaveCount(1)

  // A second tab's save reaches this tab's open list.
  const other = await context.newPage()
  await other.goto('/')
  const otherInput = other.getByRole('combobox', { name: 'Message #general' })
  await expect(otherInput).toHaveValue('older text from tab A')
  await otherInput.fill('NEWER text typed in tab B')
  await expect(rows(page).filter({ hasText: 'NEWER text typed in tab B' })).toHaveCount(1)
  await other.close()

  // A tab that missed a save (a write in this document raises no storage event here) still
  // checks storage before discarding: the stale copy's discard is refused and the text kept.
  const row = rows(page).filter({ hasText: 'NEWER text typed in tab B' })
  await page.evaluate(
    (k) => {
      const drafts = JSON.parse(localStorage.getItem(k)!)
      for (const id of Object.keys(drafts)) drafts[id] = 'NEWEST text saved elsewhere'
      localStorage.setItem(k, JSON.stringify(drafts))
    },
    (await stored(page)).key,
  )
  await expect(row).toHaveCount(1)
  await row.getByRole('button', { name: /^Discard draft/ }).click()
  await expect(dialog(page).getByRole('status')).toContainText('changed since it was listed')
  await expect(rows(page).filter({ hasText: 'NEWEST text saved elsewhere' })).toHaveCount(1)
  expect(Object.values((await stored(page)).drafts)).toEqual(['NEWEST text saved elsewhere'])
  await page.keyboard.press('Escape')
  await expect(input).toHaveValue('NEWEST text saved elsewhere')
  await page.reload()
  await expect(input).toHaveValue('NEWEST text saved elsewhere')
})
