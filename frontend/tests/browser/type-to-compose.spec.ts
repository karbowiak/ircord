import { test, expect, type Page } from '@playwright/test'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const composer = (page: Page, name = '#general') =>
  page.getByRole('combobox', { name: `Message ${name}` })

/** Leaves focus on the page itself, as after clicking empty space, with no text selected. */
async function blurAll(page: Page) {
  const body = await page.evaluate(() => {
    ;(document.activeElement as HTMLElement | null)?.blur()
    window.getSelection()?.removeAllRanges()
    return document.activeElement === document.body
  })
  expect(body).toBe(true)
}

/** Sends one real browser key event carrying text, as a non-US layout or emoji keyboard does. */
async function typeKey(page: Page, key: string, code = '') {
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('Input.dispatchKeyEvent', { type: 'keyDown', key, code, text: key })
  await cdp.send('Input.dispatchKeyEvent', { type: 'keyUp', key, code })
  await cdp.detach()
}

test('typing on the unfocused workspace or sidebar goes to the composer at the remembered caret', async ({
  page,
}) => {
  const input = composer(page)
  await input.fill('helloworld')
  await input.evaluate((element: HTMLTextAreaElement) => element.setSelectionRange(5, 5))
  await blurAll(page)
  await page.keyboard.type(', ')
  await expect(input).toBeFocused()
  await expect(input).toHaveValue('hello, world')

  // From a focused sidebar link: the first key arrives exactly once and nothing is sent.
  await page.locator('.conversation-link').filter({ hasText: 'general' }).focus()
  await page.keyboard.type('x')
  await expect(input).toHaveValue('hello, xworld')
  await expect(page.locator('.message-text').filter({ hasText: 'xworld' })).toHaveCount(0)
})

test('typing follows the open conversation without moving drafts or duplicating keys', async ({
  page,
}) => {
  await composer(page).fill('general draft')
  for (const name of ['development', 'general', 'development']) {
    await page.locator('.conversation-link').filter({ hasText: name }).click()
    await expect(composer(page, `#${name}`)).toBeEnabled()
  }
  await blurAll(page)
  await page.keyboard.type('a')
  await expect(composer(page, '#development')).toHaveValue('a')
  await page.locator('.conversation-link').filter({ hasText: 'general' }).click()
  await expect(composer(page)).toHaveValue('general draft')
})

test('international, emoji and IME input start in the composer', async ({ page }) => {
  const input = composer(page)
  await blurAll(page)
  await typeKey(page, 'é', 'Digit2')
  await expect(input).toHaveValue('é')
  await blurAll(page)
  await typeKey(page, '😀')
  await expect(input).toHaveValue('é😀')

  // An IME announces its composition with a “Process” key before any text exists.
  await blurAll(page)
  const cdp = await page.context().newCDPSession(page)
  await cdp.send('Input.dispatchKeyEvent', {
    type: 'rawKeyDown',
    key: 'Process',
    code: 'KeyN',
    windowsVirtualKeyCode: 229,
  })
  await expect(input).toBeFocused()
  await cdp.send('Input.imeSetComposition', { text: 'に', selectionStart: 1, selectionEnd: 1 })
  await cdp.send('Input.insertText', { text: 'に' })
  await expect(input).toHaveValue('é😀に')

  // A dead key (´ then e) composes its accent in the composer.
  await blurAll(page)
  await cdp.send('Input.dispatchKeyEvent', { type: 'rawKeyDown', key: 'Dead', code: 'Equal' })
  await expect(input).toBeFocused()
  await cdp.send('Input.imeSetComposition', { text: '´', selectionStart: 1, selectionEnd: 1 })
  await cdp.send('Input.insertText', { text: 'é' })
  await cdp.detach()
  await expect(input).toHaveValue('é😀にé')
})

test('shortcuts, overlays and other controls keep their keys', async ({ page }) => {
  const input = composer(page)
  await blurAll(page)

  // ? opens help; the shortcuts dialog keeps the keyboard even with a button focused.
  await page.keyboard.press('?')
  const help = page.getByRole('dialog', { name: 'Keyboard shortcuts' })
  await expect(help).toBeVisible()
  await help.getByRole('button').first().focus()
  await page.keyboard.type('ab')
  await expect(help).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(help).toBeHidden()

  // Ctrl/⌘ combinations are shortcuts, not text.
  await blurAll(page)
  await page.keyboard.press('ControlOrMeta+g')
  await page.keyboard.press('ControlOrMeta+k')
  const search = page.getByRole('textbox', { name: 'Search workspace' })
  await expect(search).toBeFocused()
  await page.keyboard.type('coffee')
  await expect(search).toHaveValue('coffee')
  await page.keyboard.press('Escape')

  // A context menu keeps type-ahead keys, ? and Alt-arrows while open.
  await page.locator('.member-button').filter({ hasText: 'Noah' }).click({ button: 'right' })
  const menu = page.getByRole('menu')
  await expect(menu).toBeVisible()
  await page.keyboard.type('g')
  await page.keyboard.press('?')
  await page.keyboard.press('Alt+ArrowDown')
  await expect(menu).toBeVisible()
  await expect(help).toBeHidden()
  await expect(input).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(menu).toBeHidden()

  // The expression picker keeps keys on its own controls.
  await page.getByRole('button', { name: 'Choose an emoji' }).click()
  const picker = page.getByRole('dialog', { name: /picker/ })
  await expect(picker).toBeVisible()
  await picker.getByRole('separator', { name: 'Resize picker' }).focus()
  await page.keyboard.type('z')
  await page.keyboard.press('?')
  await expect(picker).toBeVisible()
  await expect(help).toBeHidden()
  await page.keyboard.press('Escape')

  // Space still pages the focused timeline instead of starting a message.
  const timeline = page.locator('.timeline-scroll')
  await timeline.evaluate((element) => {
    element.scrollTop = 0
    ;(element as HTMLElement).focus()
  })
  await page.keyboard.press(' ')
  await expect(input).not.toBeFocused()
  await expect.poll(() => timeline.evaluate((element) => element.scrollTop)).toBeGreaterThan(0)

  // Space still presses a focused button.
  await page.getByRole('button', { name: 'Search messages', exact: true }).focus()
  await page.keyboard.press(' ')
  await expect(search).toBeVisible()
  await page.keyboard.press('Escape')

  await expect(input).toHaveValue('')
})

test('selected message text stays selected and a disabled composer is left alone', async ({
  page,
}) => {
  const input = composer(page)
  const text = page.locator('.message-text').first()
  await text.evaluate((element) => {
    const range = document.createRange()
    range.selectNodeContents(element)
    window.getSelection()?.removeAllRanges()
    window.getSelection()?.addRange(range)
  })
  const selected = await page.evaluate(() => window.getSelection()?.toString())
  expect(selected).toBeTruthy()
  await page.keyboard.type('q')
  await expect(input).not.toBeFocused()
  await expect(input).toHaveValue('')
  expect(await page.evaluate(() => window.getSelection()?.toString())).toBe(selected)

  const network = await page.evaluate(async () => {
    const bootstrap = await (await fetch('/api/v1/bootstrap')).json()
    const general = bootstrap.conversations.find((c: { name: string }) => c.name === 'general')
    return general.networkId as string
  })
  await page.request.patch(`/api/v1/networks/${network}`, { data: { status: 'disconnected' } })
  await expect(input).toBeDisabled()
  // The handler skips a disabled composer; the browser would also refuse to focus it.
  await blurAll(page)
  await page.keyboard.type('q')
  await expect(input).toHaveValue('')
  await page.request.patch(`/api/v1/networks/${network}`, { data: { status: 'connected' } })
  await expect(input).toBeEnabled()
})

test('fields and keyed widgets outside any overlay keep their keys', async ({ page }) => {
  // The server window's line filter sits in the main column, not in a dialog.
  await page
    .getByRole('complementary', { name: 'Conversations' })
    .getByRole('button', { name: /Server/ })
    .click()
  const raw = page.getByRole('combobox', { name: /Command to/ })
  await expect(raw).toBeEnabled()
  const filter = page.getByRole('textbox', { name: 'Filter server lines' })
  await filter.click()
  await page.keyboard.type('MOTD')
  await expect(filter).toHaveValue('MOTD')
  await expect(raw).toHaveValue('')

  // A focused widget with its own type-ahead (a radio group) keeps letters too.
  await page.evaluate(() => {
    const group = document.createElement('div')
    group.setAttribute('role', 'radiogroup')
    group.setAttribute('aria-label', 'Probe')
    group.tabIndex = 0
    document.querySelector('.chat-main')!.prepend(group)
    group.focus()
  })
  await page.keyboard.type('a')
  await expect(page.getByRole('radiogroup', { name: 'Probe' })).toBeFocused()
  await expect(raw).toHaveValue('')
})
