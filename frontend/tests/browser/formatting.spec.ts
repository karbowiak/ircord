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
    await expect(block.locator('.code-lang')).toHaveText('js')
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

  const results = await new AxeBuilder({ page }).include('pre.code-block').analyze()
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
