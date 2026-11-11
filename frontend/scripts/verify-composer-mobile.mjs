// Disposable browser fixtures: HTTP and WebSocket requests never reach an IRC server.
import assert from 'node:assert/strict'
import { mkdir } from 'node:fs/promises'
import { chromium, devices, webkit } from 'playwright'
import { createFixtures } from '../server/utils/fixtures.ts'
import { defaultNotificationSettings } from '../shared/notifications.ts'
import { emptyLibrary } from '../shared/library.ts'

const base = process.env.COMPOSER_TEST_BASE || 'http://127.0.0.1:4180'
assert(['127.0.0.1', 'localhost'].includes(new URL(base).hostname))
const engine = process.env.COMPOSER_TEST_BROWSER === 'webkit' ? webkit : chromium
const browser = await engine.launch({ headless: true })
const errors = []
const fixtures = createFixtures()
const snapshot = fixtures.bootstrap
snapshot.demo = true
const conversation = snapshot.conversations.find((c) => c.id === 'general')
const network = snapshot.networks.find((n) => n.id === conversation.networkId)
const probe = process.argv.includes('--probe-focus')
let page
let socket
let seq = snapshot.cursor
let releaseEmoji
const emojiGate = new Promise((resolve) => (releaseEmoji = resolve))
let emojiRequested = false
let holdReady = false
let connections = 0
const sent = []
const emoji = {
  id: '1F604',
  shortcode: 'smile',
  label: 'smiling face',
  aliases: [],
  group: 'smileys-emotion',
  unicode: '😄',
  source: 'unicode',
}
const context = await browser.newContext({
  ...devices[engine === webkit ? 'iPhone 13' : 'Pixel 7'],
  serviceWorkers: 'block',
})
function event(type, data) {
  socket.send(JSON.stringify({ v: 1, seq: ++seq, type, data }))
}
try {
  await context.route('**/*', (route) =>
    new URL(route.request().url()).origin === base ? route.continue() : route.abort(),
  )
  await context.routeWebSocket('**/api/v1/events**', (connection) => {
    socket = connection
    connections++
    if (!holdReady) connection.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: seq }))
  })
  await context.route('**/data/emoji-v18.json', async (route) => {
    emojiRequested = true
    await emojiGate
    await route.fulfill({ json: [emoji] })
  })
  await context.route('**/api/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (!path.startsWith('/api/')) return route.fallback()
    let body = {}
    if (path.endsWith('/auth/session'))
      body = { account: { id: 'composer-fixture', username: 'composer-fixture' } }
    else if (path.endsWith('/bootstrap')) body = snapshot
    else if (path.endsWith('/notifications/settings')) body = defaultNotificationSettings
    else if (path.endsWith('/messages') && request.method() === 'POST') {
      const message = request.postDataJSON()
      sent.push(message)
      body = { clientId: message.clientId }
    } else if (path.includes('/messages')) body = { items: [], nextCursor: null }
    else if (path.endsWith('/inbox/unread')) body = { unread: 0 }
    else if (path.endsWith('/saved-message-ids')) body = { items: [] }
    else if (path.endsWith('/app/appearance')) body = { settings: {} }
    else if (path.endsWith('/app/preferences')) body = { preferences: {} }
    else if (path.endsWith('/app/library')) body = emptyLibrary()
    else if (path.endsWith('/emotes')) body = { items: [], version: 0 }
    else if (path.endsWith('/emoji/catalog')) body = { items: [] }
    await route.fulfill({ json: body })
  })
  page = await context.newPage()
  page.setDefaultTimeout(10000)
  page.on('pageerror', (error) => errors.push(error.stack))
  await page.goto(`${base}/channel/general`)
  const field = page.locator('.composer textarea')
  await field.waitFor()
  const gif = page.getByRole('button', { name: 'Choose a GIF', exact: true })
  const emojiButton = page.getByRole('button', { name: 'Choose an emoji', exact: true })
  const send = page.getByRole('button', { name: 'Send message', exact: true })
  async function preserved(expected) {
    const actual = await field.evaluate((el) => ({
      same: el === window.composerFixture.field,
      focused: document.activeElement === el,
      disabled: el.disabled,
      value: el.value,
      selection: [el.selectionStart, el.selectionEnd, el.selectionDirection],
      trace: [...window.composerFixture.trace],
    }))
    assert.deepEqual(actual, { same: true, focused: true, disabled: false, trace: [], ...expected })
  }
  if (!probe) {
    assert(await gif.isVisible())
    assert(await emojiButton.isVisible())
  }
  await field.fill('Draft survives a network update')
  await field.evaluate((el) => {
    window.composerFixture = { field: el, trace: [] }
    el.setSelectionRange(6, 14, 'backward')
    el.addEventListener('blur', () => window.composerFixture.trace.push('blur'))
    new MutationObserver((records) => {
      if (records.some((record) => record.attributeName === 'disabled'))
        window.composerFixture.trace.push(`disabled:${el.disabled}`)
    }).observe(el, { attributes: true, attributeFilter: ['disabled'] })
  })
  event('network.updated', { ...network, status: 'disconnected' })
  await page.waitForFunction(() =>
    document.querySelector('.composer').classList.contains('composer-disabled'),
  )
  const disconnected = await field.evaluate((el) => ({
    same: el === window.composerFixture.field,
    focused: document.activeElement === el,
    disabled: el.disabled,
    value: el.value,
    selection: [el.selectionStart, el.selectionEnd, el.selectionDirection],
    trace: window.composerFixture.trace,
  }))
  if (!probe) {
    assert(await send.isDisabled())
    await field.press('Enter')
    assert.equal(sent.length, 0, 'Enter cannot submit while sending is unavailable')
  }
  event('network.updated', network)
  await page.waitForFunction(
    () => !document.querySelector('.composer').classList.contains('composer-disabled'),
  )
  if (probe) {
    console.log(
      JSON.stringify(
        {
          engine: engine.name(),
          emulation: 'mobile',
          disconnected,
          reconnectedFocused: await field.evaluate((el) => document.activeElement === el),
          errors,
        },
        null,
        2,
      ),
    )
  } else {
    assert.equal(disconnected.focused, true, 'Network status updates must not dismiss the editor')
    assert.equal(
      disconnected.disabled,
      false,
      'Draft editing remains available while sending is unavailable',
    )
    assert.equal(disconnected.same, true)
    assert.equal(disconnected.value, 'Draft survives a network update')
    assert.deepEqual(disconnected.selection, [6, 14, 'backward'])
    assert.deepEqual(disconnected.trace, [])

    // A focused nonempty mobile draft hides only the two expression controls, without remounts.
    assert.equal(await gif.isVisible(), false)
    assert.equal(await emojiButton.isVisible(), false)
    const editingWidth = await field.evaluate((el) => el.getBoundingClientRect().width)
    await preserved({ value: 'Draft survives a network update', selection: [6, 14, 'backward'] })

    // Synthetic composition events cover DOM and reactive state, not a native software keyboard.
    const composing = await field.evaluate((el) => {
      el.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
      el.setRangeText('に', 6, 14, 'end')
      el.dispatchEvent(
        new InputEvent('input', {
          bubbles: true,
          isComposing: true,
          inputType: 'insertCompositionText',
          data: 'に',
        }),
      )
      return {
        value: el.value,
        selection: [el.selectionStart, el.selectionEnd, el.selectionDirection],
      }
    })
    assert(emojiRequested, 'Focusing the editor starts lazy media loading')
    releaseEmoji()
    await page.waitForResponse('**/data/emoji-v18.json')
    event('message.created', {
      ...fixtures.messages.general[0],
      id: 'composer-incoming',
      senderId: 'jules',
      kind: 'message',
      text: 'Disposable incoming fixture',
      attachment: undefined,
      gif: undefined,
    })
    await page.getByText('Disposable incoming fixture', { exact: true }).waitFor()
    socket.send(
      JSON.stringify({
        v: 1,
        type: 'typing',
        data: { conversationId: 'general', memberId: 'jules', state: 'active' },
      }),
    )
    await page.locator('.typing-line').waitFor()
    await page.setViewportSize({ width: 430, height: 520 })
    await preserved(composing)
    event('network.updated', { ...network, status: 'disconnected' })
    await page.waitForFunction(() =>
      document.querySelector('.composer').classList.contains('composer-disabled'),
    )
    await preserved(composing)
    event('network.updated', network)
    await page.waitForFunction(
      () => !document.querySelector('.composer').classList.contains('composer-disabled'),
    )
    await preserved(composing)
    await field.evaluate((el) =>
      el.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true, data: 'に' })),
    )
    await field.pressSequentially(' committed')
    assert.equal(
      await field.inputValue(),
      `${composing.value.slice(0, composing.selection[0])} committed${composing.value.slice(composing.selection[1])}`,
    )

    // A transport reconnect also keeps the same editor; deliberate blur remains deliberate.
    holdReady = true
    const previousConnections = connections
    socket.close()
    await page.waitForFunction(() =>
      document.querySelector('.composer').classList.contains('composer-disabled'),
    )
    assert(await field.evaluate((el) => el === document.activeElement))
    await page.waitForTimeout(300)
    for (let tries = 0; connections === previousConnections; tries++) {
      assert(tries < 50, 'Socket reconnect starts')
      await page.waitForTimeout(100)
    }
    socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: seq }))
    holdReady = false
    await page.waitForFunction(
      () => !document.querySelector('.composer').classList.contains('composer-disabled'),
    )
    assert(await field.evaluate((el) => el === document.activeElement))
    await page.setViewportSize(devices[engine === webkit ? 'iPhone 13' : 'Pixel 7'].viewport)
    await mkdir('../.data/composer-mobile', { recursive: true })
    await page.screenshot({ path: `../.data/composer-mobile/${engine.name()}-editing.png` })
    await field.blur()
    assert(await gif.isVisible())
    assert(await emojiButton.isVisible())
    assert((await field.evaluate((el) => el.getBoundingClientRect().width)) < editingWidth)
    event('network.updated', { ...network, status: 'disconnected' })
    await page.waitForFunction(() =>
      document.querySelector('.composer').classList.contains('composer-disabled'),
    )
    event('network.updated', network)
    await page.waitForFunction(
      () => !document.querySelector('.composer').classList.contains('composer-disabled'),
    )
    assert.equal(await field.evaluate((el) => el === document.activeElement), false)
    await page.screenshot({ path: `../.data/composer-mobile/${engine.name()}-idle.png` })

    // Restored controls accept a pointer/touch click; choosing emoji restores the caret.
    await emojiButton.tap()
    await page.locator('.expression-search input').waitFor()
    await page.locator('.expression-search input').fill('smile')
    await page.locator('.expression-picker [data-emoji-index="0"]').tap()
    await page.locator('.expression-picker').waitFor({ state: 'hidden' })
    assert(await field.evaluate((el) => el === document.activeElement))
    assert((await field.inputValue()).includes('😄'))
    assert.equal(await emojiButton.isVisible(), false)
    await field.blur()
    await gif.tap()
    await page.locator('.expression-picker').waitFor()
    const pickerState = (tab) =>
      page.waitForFunction(
        (tab) => window.history.state?.ircordViews?.values['composer-picker'] === tab,
        tab,
      )
    await pickerState('gifs')
    await page.getByRole('tab', { name: 'Emoji', exact: true }).tap()
    await pickerState('emoji')
    await page.goBack()
    await page.getByRole('tab', { name: 'GIFs', selected: true, exact: true }).waitFor()
    await page.goBack()
    await page.locator('.expression-picker').waitFor({ state: 'hidden' })
    assert(await gif.evaluate((el) => el === document.activeElement))
    await page.goForward()
    await page.getByRole('tab', { name: 'GIFs', selected: true, exact: true }).waitFor()
    await page.goForward()
    await page.getByRole('tab', { name: 'Emoji', selected: true, exact: true }).waitFor()
    assert.equal(new URL(page.url()).pathname, '/channel/general')
    await page.keyboard.press('Escape')
    await page.locator('.expression-picker').waitFor({ state: 'hidden' })
    assert(await emojiButton.evaluate((el) => el === document.activeElement))
    await field.fill('Keyboard picker draft')
    await field.evaluate((el) => el.setSelectionRange(2, 7, 'backward'))
    await field.press('Meta+g')
    await page.locator('.expression-search input').waitFor()
    await page.keyboard.press('Escape')
    await page.locator('.expression-picker').waitFor({ state: 'hidden' })
    await page.waitForFunction(
      () => document.activeElement === document.querySelector('.composer textarea'),
    )
    assert.deepEqual(
      await field.evaluate((el) => [
        el.value,
        el.selectionStart,
        el.selectionEnd,
        el.selectionDirection,
      ]),
      ['Keyboard picker draft', 2, 7, 'backward'],
    )

    // Autocomplete and type-to-compose keep their own focus rules.
    await field.fill(':sm')
    await page.locator('.composer-suggestions').waitFor()
    assert(await field.evaluate((el) => el === document.activeElement))
    await field.press('Enter')
    assert.equal(await field.inputValue(), '😄')
    await field.fill('')
    assert(await gif.isVisible())
    assert(await emojiButton.isVisible())
    await field.pressSequentially('Fixture send')
    await send.tap()
    assert.equal(await field.inputValue(), '')
    assert(await field.evaluate((el) => el === document.activeElement))
    assert(await gif.isVisible())
    assert(await emojiButton.isVisible())
    await field.blur()
    await page.keyboard.type('x')
    assert.equal(await field.inputValue(), 'x')

    // The same draft at desktop widths retains both controls and the same textarea.
    await page.setViewportSize({ width: 1440, height: 900 })
    assert(await gif.isVisible())
    assert(await emojiButton.isVisible())
    assert(
      await field.evaluate(
        (el) => el === window.composerFixture.field && el === document.activeElement,
      ),
    )
    // WebKit on macOS uses Option+Tab to include buttons in native Tab traversal.
    await field.press(engine === webkit ? 'Alt+Tab' : 'Tab')
    assert(await gif.evaluate((el) => el === document.activeElement))
    await field.fill('Desktop draft')
    await field.evaluate((el) => el.setSelectionRange(2, 7, 'backward'))
    await field.press('Meta+,')
    const settings = page.getByRole('dialog', { name: 'Settings', exact: true })
    await settings.waitFor()
    await settings.getByRole('button', { name: 'Close dialog', exact: true }).click()
    await settings.waitFor({ state: 'hidden' })
    await page.waitForFunction(
      () => document.activeElement === document.querySelector('.composer textarea'),
    )
    assert.deepEqual(
      await field.evaluate((el) => [
        el.value,
        el.selectionStart,
        el.selectionEnd,
        el.selectionDirection,
      ]),
      ['Desktop draft', 2, 7, 'backward'],
    )
    const sidebar = page.locator('.conversation-sidebar')
    await sidebar.getByText('design', { exact: true }).click()
    await page.waitForURL(`${base}/channel/design`)
    assert.equal(await field.inputValue(), '')
    assert.equal(await field.evaluate((el) => el === document.activeElement), false)
    await sidebar.getByText('general', { exact: true }).click()
    await page.waitForURL(`${base}/channel/general`)
    assert.equal(await field.inputValue(), 'Desktop draft')
    assert.equal(await field.evaluate((el) => el === document.activeElement), false)
    assert.deepEqual(errors, [])
    console.log(
      `${engine.name()} mobile emulation: controls, draft, selection, synthetic composition, lazy media, incoming messages, typing, network/socket reconnect, intentional blur, picker taps/Back/Forward, autocomplete, send, dialog return, conversation changes and desktop keyboard checks passed.`,
    )
  }
} catch (error) {
  console.error({ url: page?.url(), body: await page?.locator('body').innerText(), errors })
  throw error
} finally {
  releaseEmoji()
  await context.close()
  await browser.close()
}
