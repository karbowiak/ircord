import { test, expect, type Page } from '@playwright/test'

// The service worker is what this file tests; the other specs block it.
test.use({ serviceWorkers: 'allow' })

const SHELL = 'ircord-shell-v1'

async function controlled(page: Page) {
  await page.goto('/')
  await page.evaluate(() => navigator.serviceWorker.ready)
  // The worker claims open pages on activation; a reload covers a slow claim.
  if (!(await page.evaluate(() => !!navigator.serviceWorker.controller))) await page.reload()
  await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true)
}

// Seeds the shell cache as an older release would have left it.
const seed = (page: Page, path: string, body: string, type: string) =>
  page.evaluate(
    async ({ shell, path, body, type }) =>
      (await caches.open(shell)).put(
        path,
        new Response(body, { headers: { 'content-type': type } }),
      ),
    { shell: SHELL, path, body, type },
  )
const cached = (page: Page, path: string) =>
  page.evaluate(
    async ({ shell, path }) => (await (await caches.open(shell)).match(path))?.text() ?? null,
    { shell: SHELL, path },
  )
const fetched = (page: Page, path: string) =>
  page.evaluate(async (path) => (await fetch(path)).text(), path)

test('a changed favicon replaces the cached one on the next load instead of sticking', async ({
  page,
}) => {
  await controlled(page)
  const stale = '<svg xmlns="http://www.w3.org/2000/svg"><title>old mark</title></svg>'
  await seed(page, '/favicon.svg', stale, 'image/svg+xml')

  // Served at once from the cache, then refreshed from the server in the background.
  expect(await fetched(page, '/favicon.svg')).toBe(stale)
  await expect.poll(() => cached(page, '/favicon.svg')).toContain('#b8a4f8')
  expect(await fetched(page, '/favicon.svg')).toContain('#b8a4f8')
})

test('an icon URL the server no longer has keeps its cached image, not the HTML page', async ({
  page,
}) => {
  await controlled(page)
  await seed(page, '/icons/retired.png', 'old png bytes', 'image/png')
  expect(await fetched(page, '/icons/retired.png')).toBe('old png bytes')
  // The background refresh is answered with the server's HTML fallback page; let it settle.
  await page.waitForTimeout(1000)
  expect(await cached(page, '/icons/retired.png')).toBe('old png bytes')
})

test('avatars added to the default avatar list appear on the next load instead of sticking', async ({
  page,
}) => {
  await controlled(page)
  const path = '/avatars/defaults/manifest.json'
  const stale = JSON.stringify({
    schema: 1,
    avatars: [{ id: 'avatar-001', file: 'avatar-001.png', category: 'male' }],
  })
  await seed(page, path, stale, 'application/json')
  // An older copy in the media cache must not win either.
  await page.evaluate(
    async ({ path, stale }) =>
      (await caches.open('ircord-media-v2')).put(
        path,
        new Response(stale, { headers: { 'content-type': 'application/json' } }),
      ),
    { path, stale },
  )
  const choices = () =>
    page.getByRole('radiogroup', { name: 'Default profile pictures' }).getByRole('radio')
  // The next load starts from the cached list and refreshes it in the background ...
  await page.reload()
  await page.getByRole('button', { name: 'Create an account' }).click()
  await expect(choices()).toHaveCount(1)
  await expect.poll(() => cached(page, path)).toContain('avatar-032')
  // ... so the load after that offers the server's full set.
  await page.reload()
  await page.getByRole('button', { name: 'Create an account' }).click()
  await expect(choices()).toHaveCount(32)
})
