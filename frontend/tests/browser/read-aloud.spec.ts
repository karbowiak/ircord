import { test, expect, type Page, type Route } from '@playwright/test'
import { signIn } from './helpers'

// A fake OpenAI-compatible speech service, answered in the browser: tests never call a provider.
const service = 'https://tts.example.test'
const speechUrl = `${service}/v1/audio/speech`

/** A quarter-second quiet tone as WAV. */
function tone() {
  const samples = 2000
  const b = Buffer.alloc(44 + samples)
  b.write('RIFF', 0)
  b.writeUInt32LE(36 + samples, 4)
  b.write('WAVEfmt ', 8)
  b.writeUInt32LE(16, 16)
  b.writeUInt16LE(1, 20)
  b.writeUInt16LE(1, 22)
  b.writeUInt32LE(8000, 24)
  b.writeUInt32LE(8000, 28)
  b.writeUInt16LE(1, 32)
  b.writeUInt16LE(8, 34)
  b.write('data', 36)
  b.writeUInt32LE(samples, 40)
  for (let i = 0; i < samples; i++) b[44 + i] = 128 + Math.round(20 * Math.sin(i / 3))
  return b
}

interface Heard {
  input: string
  authorization?: string
  cookie?: string
}
/** Serves speech for every page of the context, with or without CORS, recording each request. */
async function fakeSpeech(page: Page, { cors = true } = {}) {
  const heard: Heard[] = []
  const allow: Record<string, string> = cors
    ? {
        'Access-Control-Allow-Origin': '*',
        'Access-Control-Allow-Headers': 'authorization, content-type',
        'Access-Control-Allow-Methods': 'POST',
      }
    : {}
  await page.context().route(speechUrl, async (route: Route) => {
    const request = route.request()
    if (request.method() === 'OPTIONS') return route.fulfill({ status: 204, headers: allow })
    const headers = await request.allHeaders()
    heard.push({
      input: request.postDataJSON().input,
      authorization: headers.authorization,
      cookie: headers.cookie,
    })
    await route.fulfill({
      status: 200,
      headers: { ...allow, 'Content-Type': 'audio/wav' },
      body: tone(),
    })
  })
  return heard
}

const connectSrc = (csp: string | undefined) =>
  csp
    ?.split(';')
    .map((d) => d.trim())
    .find((d) => d.startsWith('connect-src ')) ?? ''

async function setUp(page: Page, key = 'sk-demo-not-a-real-key') {
  await page.getByRole('button', { name: 'Read highlights aloud' }).click()
  const content = page.locator('.settings-content')
  await expect(content.getByRole('heading', { name: 'Read aloud' })).toBeVisible()
  await content.getByLabel('Service URL').fill(`${service}/v1`)
  if (key) await content.getByLabel('API key').fill(key)
  await content.getByLabel('Model').fill('tts-1')
  await content.getByLabel('Voice').fill('alloy')
  const saved = page.waitForResponse(
    (r) => r.url().includes('/api/app/preferences') && r.request().method() === 'PUT' && r.ok(),
  )
  await content.getByRole('button', { name: 'Save', exact: true }).click()
  await saved
  await expect(
    content.getByText('Saved. Reload IRCord to connect to the new address.'),
  ).toBeVisible()
  return content
}

async function reloadAndOpen(page: Page) {
  await page.locator('.settings-content').getByRole('button', { name: 'Reload', exact: true }).click()
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeEnabled()
  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.locator('.settings-nav').getByRole('button', { name: 'Read aloud', exact: true }).click()
  return page.locator('.settings-content')
}

test('read-aloud: the browser calls your own service, and highlights are read only while on', async ({
  page,
}) => {
  const heard = await fakeSpeech(page)
  // The key never goes to IRCord's own server, in any request.
  const toIrcord: string[] = []
  page.context().on('request', (r) => {
    if (!r.url().startsWith(service))
      toIrcord.push(`${r.url()} ${JSON.stringify(r.headers())} ${r.postData() ?? ''}`)
  })
  await signIn(page)
  const toggle = page.getByRole('button', { name: 'Read highlights aloud' })
  await expect(toggle).toHaveAttribute('aria-pressed', 'false')

  // Without a service, the toggle opens its settings instead of switching on.
  let content = await setUp(page)
  await expect(content.getByLabel('API key')).toHaveValue('')
  await expect(content.getByLabel('API key')).toHaveAttribute('placeholder', /Saved on this device/)
  // The page's policy allows the new address only after a reload.
  content = await reloadAndOpen(page)
  await expect(content.getByLabel('Service URL')).toHaveValue(`${service}/v1`)
  await content.getByRole('button', { name: 'Test', exact: true }).click()
  await expect(content.getByText('Played a sample.')).toBeVisible()
  expect(heard.at(-1)).toEqual({
    input: 'This is how IRCord reads your highlights aloud.',
    authorization: 'Bearer sk-demo-not-a-real-key',
    cookie: undefined,
  })

  await content.getByRole('switch', { name: 'Read highlights aloud' }).click()
  await page.getByRole('button', { name: 'Close dialog' }).click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')

  // The echo bot repeats what you send: your own copy is never read, the echo that mentions
  // you is, and one without a highlight is not.
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await input.fill('plain chatter, nothing to see')
  await input.press('Enter')
  await input.fill('karbowiak, read this one')
  await input.press('Enter')
  await expect
    .poll(() => heard.map((h) => h.input))
    .toContain('echo in #general: karbowiak, read this one')
  expect(heard.filter((h) => h.input.includes('read this one'))).toHaveLength(1)
  expect(heard.some((h) => h.input.includes('plain chatter'))).toBe(false)

  // The toggle is per device: a second tab follows it, so switching off in one tab silences both.
  const other = await page.context().newPage()
  await other.goto('/')
  const otherToggle = other.getByRole('button', { name: 'Read highlights aloud' })
  await expect(otherToggle).toHaveAttribute('aria-pressed', 'true')
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'false')
  await expect(otherToggle).toHaveAttribute('aria-pressed', 'false')
  await input.fill('karbowiak, stay quiet')
  await input.press('Enter')
  await expect(page.locator('.message-row.is-mention').filter({ hasText: 'stay quiet' })).toHaveCount(1)
  await expect(other.locator('.message-row.is-mention').filter({ hasText: 'stay quiet' })).toHaveCount(1)
  await page.waitForTimeout(500)
  expect(heard.some((h) => h.input.includes('stay quiet'))).toBe(false)

  expect(toIrcord.filter((r) => r.includes('sk-demo-not-a-real-key'))).toEqual([])
})

test('only the user who set up a service gets its origin in their page policy', async ({
  page,
  browser,
}) => {
  await signIn(page)
  const before = connectSrc((await page.request.get('/')).headers()['content-security-policy'])
  expect(before).toContain("connect-src 'self'")
  expect(before).not.toContain(service)
  await setUp(page, '')
  const mine = await page.request.get('/')
  const mineSrc = connectSrc(mine.headers()['content-security-policy'])
  expect(mineSrc.split(/\s+/)).toContain(service)
  expect(mine.headers()['cache-control']).toContain('private')

  // Someone else, in their own browser, never gets it; nor does a signed-out visitor.
  const otherContext = await browser.newContext()
  const other = await otherContext.newPage()
  await signIn(other)
  const theirs = connectSrc((await other.request.get('/')).headers()['content-security-policy'])
  expect(theirs).toContain("connect-src 'self'")
  expect(theirs).not.toContain(service)
  const anonymous = await browser.newContext()
  const signedOut = connectSrc(
    (await anonymous.request.get('/')).headers()['content-security-policy'],
  )
  expect(signedOut).not.toContain(service)
  await otherContext.close()
  await anonymous.close()

  // A bare scheme saved straight to the preferences route is dropped, never added to the policy.
  const saved = await page.request.put('/api/app/preferences', {
    data: { preferences: { speech: { baseUrl: 'https:', model: 'tts-1', voice: 'alloy' } } },
  })
  expect(
    ((await saved.json()) as { preferences: { speech: { baseUrl: string } } }).preferences.speech
      .baseUrl,
  ).toBe('')
  const after = connectSrc((await page.request.get('/')).headers()['content-security-policy'])
  for (const policy of [before, mineSrc, theirs, signedOut, after]) {
    const tokens = policy.split(/\s+/)
    expect(tokens).not.toContain('https:')
    expect(tokens).not.toContain('http:')
    expect(tokens).not.toContain('*')
  }
  expect(after).not.toContain(service)
})

test('a service that doesn’t allow browsers is explained as CORS in Test', async ({ page }) => {
  await fakeSpeech(page, { cors: false })
  await signIn(page)
  await setUp(page)
  const content = await reloadAndOpen(page)
  await content.getByRole('button', { name: 'Test', exact: true }).click()
  await expect(content.getByText(/CORS/)).toBeVisible()
})
