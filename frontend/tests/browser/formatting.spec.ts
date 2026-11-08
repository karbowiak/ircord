import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

test.beforeEach(async ({ page }) => {
  await signIn(page)
})

const code = [
  'function f(a_b) {',
  '  // https://example.com/in_code :smile: @you #general',
  '',
  '    return "<script>alert(1)</script>" + `**not bold**`',
  '}',
].join('\n')

test('renders fenced and inline code literally, live and after reload', async ({ page }) => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  page.on('dialog', (d) => errors.push(`dialog: ${d.message()}`))
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill(
    `look:\n\`\`\`js\n${code}\n\`\`\`\nand \`https://x.test/a_b :smile:\` **bold** _it_`,
  )
  await input.press('Enter')

  const check = async () => {
    const message = page.locator('.message-text').filter({ hasText: 'look:' }).last()
    const block = message.locator('pre.code-block')
    await expect(block.locator('code')).toHaveText(code, { useInnerText: true })
    await expect(message.locator('.code-frame .code-lang')).toHaveText('js')
    // Nothing inside code is interactive or formatted.
    await expect(block.locator('a, button, img, .inline-mention, .fmt-bold, script')).toHaveCount(0)
    const inline = message.locator('.fmt-mono')
    await expect(inline).toHaveText('https://x.test/a_b :smile:')
    await expect(inline.locator('a, img')).toHaveCount(0)
    // Formatting outside code still applies.
    await expect(message.locator('.fmt-bold')).toHaveText('bold')
    await expect(message.locator('.fmt-italic')).toHaveText('it')
    return message
  }
  await check()
  await expect(page.locator('pre.code-block')).toHaveCount(2) // the demo echo answers too
  await page.reload()
  await check()

  const results = await new AxeBuilder({ page }).include('.code-frame').analyze()
  expect(
    results.violations.filter((v) => ['serious', 'critical'].includes(v.impact ?? '')),
  ).toEqual([])
  expect(errors).toEqual([])
})

test('large code stays bounded and scrolls inside the block on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  const long = Array.from({ length: 26 }, (_, i) => `${i} ${'x'.repeat(140)}`).join('\n')
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill(`\`\`\`\n${long}\n\`\`\``)
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  const block = page.locator('pre.code-block').last()
  await expect(block).toBeVisible()
  const size = await block.evaluate((el) => ({
    height: el.clientHeight,
    scrollHeight: el.scrollHeight,
    width: el.clientWidth,
    scrollWidth: el.scrollWidth,
  }))
  expect(size.height).toBeLessThan(420)
  expect(size.scrollHeight).toBeGreaterThan(size.height)
  expect(size.scrollWidth).toBeGreaterThan(size.width)
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
})

test('an unterminated fence stays ordinary text', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('```\nnot closed **bold**')
  await input.press('Enter')
  const message = page.locator('.message-text').filter({ hasText: 'not closed' }).last()
  await expect(message.locator('pre')).toHaveCount(0)
  await expect(message.locator('.fmt-bold')).toHaveText('bold')
})

test('a code block on a network without multiline keeps the draft and names the limit', async ({
  page,
}) => {
  await page.getByRole('button', { name: 'OFTC', exact: true }).click()
  const input = page.getByRole('combobox', { name: 'Message #community' })
  const draft = '```\n  two\n  lines\n```'
  await input.fill(draft)
  await input.press('Enter')
  await expect(input).toHaveValue(draft)
  await expect(page.getByText(/This network does not support multiline messages/)).toBeVisible()
  await expect(page.locator('pre.code-block')).toHaveCount(0)
  // A one-line code span still sends.
  await input.fill('`one line` is fine')
  await input.press('Enter')
  await expect(page.locator('.fmt-mono').filter({ hasText: 'one line' }).first()).toBeVisible()
})

test('media links inside code are not previewed', async ({ page }) => {
  const gif = Buffer.from('R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7', 'base64')
  await page.route('https://media.discordapp.net/**', (route) =>
    route.fulfill({ body: gif, contentType: 'image/gif' }),
  )
  const input = page.getByRole('combobox', { name: 'Message #general' })
  for (const text of [
    'inline `https://media.discordapp.net/a/inline.gif`',
    '```\nhttps://media.discordapp.net/a/fenced.gif\n```',
    'plain https://media.discordapp.net/a/plain.gif',
  ]) {
    await input.fill(text)
    await input.press('Enter')
  }
  // The plain link previews for the message and the demo echo; the links in code never do.
  const previews = page.getByRole('button', { name: 'View GIF from media.discordapp.net' })
  await expect(previews).toHaveCount(2)
  await expect(page.locator('img[src*="inline.gif"], img[src*="fenced.gif"]')).toHaveCount(0)
})

test('code is never emoji-sized, and single-line places show fences inline', async ({ page }) => {
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('`🐈`')
  await input.press('Enter')
  const message = page.locator('.message-text').filter({ hasText: /^🐈$/ }).last()
  await expect(message.locator('.fmt-mono')).toHaveText('🐈')
  await expect(message.locator('.emoji-only-message')).toHaveCount(0)

  await input.fill('/topic ```topic code```')
  await input.press('Enter')
  const header = page.locator('.chat-header')
  await expect(header.locator('.fmt-mono')).toHaveText('topic code')
  await expect(header.locator('pre')).toHaveCount(0)
})

test('tagged code is highlighted, untagged code stays plain, and copy takes the raw source', async ({
  page,
  context,
}) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill(`highlighted:\n\`\`\`js\n${code}\n\`\`\``)
  await input.press('Enter')
  // Your own message: the demo's echo bot repeats it moments later, and the checks below must
  // not move to that copy halfway through.
  const tagged = page
    .locator('.message-row.is-mine')
    .filter({ hasText: 'highlighted:' })
    .last()
    .locator('.message-text')
  const block = tagged.locator('pre.code-block')
  await expect(block.locator('code.is-highlighted .hljs-keyword').first()).toHaveText('function')
  await expect(block.locator('.hljs-comment')).toHaveText(
    '// https://example.com/in_code :smile: @you #general',
  )
  await expect(block.locator('code')).toHaveText(code, { useInnerText: true })
  await expect(block.locator('script, img, a')).toHaveCount(0)

  // Code blocks colour from the active scheme, not the plain text colour.
  const colours = await block.evaluate((el) => ({
    keyword: getComputedStyle(el.querySelector('.hljs-keyword')!).color,
    text: getComputedStyle(el).color,
  }))
  expect(colours.keyword).not.toBe(colours.text)

  const copy = tagged.getByRole('button', { name: 'Copy code' })
  // Hidden until the block is hovered or focused from the keyboard.
  await input.focus()
  await page.mouse.move(0, 0)
  await expect(copy).toHaveCSS('opacity', '0')
  await block.focus()
  await expect(copy).toHaveCSS('opacity', '1')
  await input.focus()
  await expect(copy).toHaveCSS('opacity', '0')
  await block.hover()
  await expect(copy).toHaveCSS('opacity', '1')
  await copy.click()
  await expect(tagged.getByRole('button', { name: 'Copied' })).toBeVisible()
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(code)
  // Copying neither typed nor sent anything.
  await expect(input).toHaveValue('')

  await input.fill('plain:\n```\nconst untagged = 1\n```')
  await input.press('Enter')
  const plain = page.locator('.message-text').filter({ hasText: 'plain:' }).last()
  await expect(plain.locator('pre.code-block code')).toHaveText('const untagged = 1')
  await expect(plain.locator('pre.code-block [class^="hljs-"]')).toHaveCount(0)
  await plain.locator('pre.code-block').hover()
  await plain.getByRole('button', { name: 'Copy code' }).click()
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe('const untagged = 1')
})

test('the copy button never covers code, even where it always shows on touch screens', async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 412, height: 915 },
    hasTouch: true,
    isMobile: true,
  })
  const page = await context.newPage()
  await signIn(page)
  const input = page.getByRole('combobox', { name: 'Message #general' })
  const first = `compute(${'x'.repeat(30)})`
  await input.fill(`touch:\n\`\`\`\n${first}\nsecond line\n\`\`\``)
  await page.getByRole('button', { name: 'Send message', exact: true }).click()
  const frame = page.locator('.message-text').filter({ hasText: 'touch:' }).last()
  const copy = frame.getByRole('button', { name: 'Copy code' })
  await expect(copy).toHaveCSS('opacity', '1')
  const boxes = await frame.locator('pre.code-block code').evaluate((code) => {
    const range = document.createRange()
    range.setStart(code.firstChild!, 0)
    range.setEnd(code.firstChild!, code.textContent!.indexOf('\n'))
    const line = range.getBoundingClientRect()
    const button = code.closest('.code-frame')!.querySelector('.code-copy')!.getBoundingClientRect()
    return { line: { top: line.top, bottom: line.bottom }, button: { bottom: button.bottom } }
  })
  expect(boxes.button.bottom).toBeLessThanOrEqual(boxes.line.top)
  await context.close()
})
