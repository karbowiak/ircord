import { test, expect, type Page } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import { signIn } from './helpers'

function gif(index: number, canonical = false) {
  return {
    id: index,
    slug: `hello-${index}${canonical ? '' : '--delivery-token'}`,
    title: `Hello friend ${index}`,
    file: {
      hd: {
        gif: {
          url: `https://static.klipy.com/test-${index}.gif?delivery=original`,
          width: 400,
          height: 300,
        },
      },
      sm: {
        webp: { url: `https://static.klipy.com/test-${index}.webp`, width: 160, height: 120 },
        jpg: { url: `https://static.klipy.com/test-${index}.jpg`, width: 160, height: 120 },
      },
    },
  }
}
async function mockMedia(page: Page) {
  const queries: string[] = []
  await page.route('https://api.klipy.com/**', async (route) => {
    const url = new URL(route.request().url())
    const endpoint = url.pathname.split('/gifs/')[1] || ''
    let response: unknown
    if (endpoint === 'categories')
      response = {
        result: true,
        data: {
          categories: [
            { category: 'hello', query: 'hello', preview_url: 'https://static.klipy.com/test.jpg' },
            {
              category: 'celebrate',
              query: 'celebrate',
              preview_url: 'https://static.klipy.com/test.jpg',
            },
          ],
        },
      }
    else if (endpoint === 'items')
      response = {
        result: true,
        data: (url.searchParams.get('slugs') || '')
          .split(',')
          .filter(Boolean)
          .map((slug) => gif(Number(slug.match(/hello-(\d+)/)?.[1] || 1), true)),
      }
    else if (endpoint.startsWith('share/')) response = { result: true }
    else {
      const query = url.searchParams.get('q') || ''
      if (endpoint === 'search') queries.push(query)
      if (query === 'failure') {
        await route.fulfill({ status: 503, contentType: 'application/json', body: '{}' })
        return
      }
      const page = Number(url.searchParams.get('page') || 1)
      response = {
        result: true,
        data: {
          data:
            query === 'nothinghere'
              ? []
              : [gif(page * 10 + 1), gif(page * 10 + 2), gif(page * 10 + 3)],
          current_page: page,
          has_next: page === 1 && query !== 'nothinghere',
        },
      }
    }
    await route.fulfill({ contentType: 'application/json', body: JSON.stringify(response) })
  })
  await page.route('https://static.klipy.com/**', (route) =>
    route.fulfill({ path: 'public/media/weekend.jpg', contentType: 'image/jpeg' }),
  )
  await page.route('**/api/app/emoji/catalog', (route) =>
    route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        items: [
          {
            id: 'workspace:ircord',
            label: 'IRCord',
            shortcode: 'ircord',
            aliases: ['chat'],
            group: 'workspace',
            url: '/favicon.svg',
            staticUrl: '/favicon.svg',
            source: 'workspace',
          },
        ],
        standardUrl: '/data/emoji-v18.json',
        sources: ['workspace'],
      }),
    }),
  )
  await signIn(page)
  return queries
}

test('debounces GIF search, preserves provider order and sends a GIF over WebSocket', async ({
  page,
}) => {
  const queries = await mockMedia(page)
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  const search = page.getByRole('textbox', { name: 'Search Klipy GIFs' })
  await search.fill('h')
  await search.fill('he')
  await search.fill('hello')
  await expect(page.locator('.gif-select')).toHaveCount(3)
  expect(queries).toEqual(['hello'])
  await expect(page.locator('.gif-select').first()).toHaveAttribute(
    'aria-label',
    'Send Hello friend 11 GIF',
  )
  await page.locator('.gif-select').first().click()
  await expect(page.locator('.message-gif')).toHaveCount(2)
  await expect(page.getByRole('dialog', { name: 'GIF and emoji picker' })).toHaveCount(0)
})

test('favorites are saved to the account and survive reload', async ({ page }) => {
  await mockMedia(page)
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.getByRole('button', { name: 'Trending', exact: true }).click()
  await page.locator('.gif-save').first().click()
  await expect(page.locator('.gif-save').first()).toHaveAttribute('aria-pressed', 'true')
  await page.keyboard.press('Escape')
  await page.reload()
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.getByRole('button', { name: 'Favorites', exact: true }).click()
  await expect(page.locator('.gif-select')).toHaveCount(1)
  await expect(page.locator('.gif-save').first()).toHaveAttribute('aria-pressed', 'true')
  await page.locator('.gif-save').first().click()
  await expect(page.getByText('No favorites yet', { exact: true })).toBeVisible()
})

test('emoji support includes search, selected skin tones, recent choices and custom shortcodes', async ({
  page,
}) => {
  await mockMedia(page)
  await page.getByRole('button', { name: 'Choose an emoji', exact: true }).click()
  await page.getByRole('textbox', { name: 'Search emoji' }).fill('waving hand')
  await page.getByRole('button', { name: 'Choose skin tone' }).click()
  await page.getByRole('button', { name: 'Use medium skin tone', exact: true }).click()
  await page.getByRole('button', { name: 'Insert waving hand', exact: true }).click()
  const input = page.getByRole('combobox', { name: 'Message #general' })
  await expect(input).toHaveValue('👋🏽')
  await input.press('Enter')
  await expect(page.locator('.message-text').filter({ hasText: /^👋🏽$/ })).toHaveCount(2)
  await page.getByRole('button', { name: 'Choose an emoji', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Insert waving hand', exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Server emoji', exact: true }).click()
  await page.getByRole('button', { name: 'Insert IRCord', exact: true }).click()
  await expect(input).toHaveValue(':ircord:')
  await input.press('Enter')
  await expect(page.locator('.inline-custom-emoji')).toHaveCount(2)
})

test('picker keyboard dismissal restores focus and tab arrows switch panels', async ({ page }) => {
  await mockMedia(page)
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.getByRole('tab', { name: 'GIFs', exact: true }).focus()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByRole('tab', { name: 'Emoji', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  )
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog', { name: 'GIF and emoji picker' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Choose an emoji', exact: true })).toBeFocused()
})

test('search empty and provider failure states remain recoverable', async ({ page }) => {
  await mockMedia(page)
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  const search = page.getByRole('textbox', { name: 'Search Klipy GIFs' })
  await search.fill('nothinghere')
  await expect(page.getByText('No GIFs found', { exact: true })).toBeVisible()
  await search.fill('failure')
  await expect(page.getByText('Couldn’t load GIFs', { exact: true })).toBeVisible()
  await search.fill('hello')
  await expect(page.locator('.gif-select')).toHaveCount(3)
  await page.locator('.gif-scroll').evaluate((el) => (el.scrollTop = el.scrollHeight))
  await page.locator('.gif-scroll').dispatchEvent('scroll')
  await expect(page.locator('.gif-select')).toHaveCount(6)
})

test('emoji rendering is virtualized and both picker panels pass accessibility checks', async ({
  page,
}) => {
  await mockMedia(page)
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await expect(page.locator('.gif-category')).toHaveCount(4)
  const gifs = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze()
  expect(gifs.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }))).toEqual(
    [],
  )
  await page.getByRole('tab', { name: 'Emoji', exact: true }).click()
  await page.getByRole('button', { name: 'Smileys & emotion', exact: true }).click()
  await expect(page.locator('.emoji-choice').first()).toBeVisible()
  // Smileys & emotion has ~170 entries; only visible rows plus overscan are mounted.
  expect(await page.locator('.emoji-choice').count()).toBeLessThan(150)
  const emoji = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze()
  expect(emoji.violations.map((v) => ({ id: v.id, nodes: v.nodes.map((n) => n.target) }))).toEqual(
    [],
  )
  await page.setViewportSize({ width: 390, height: 844 })
  const box = await page.locator('.expression-picker').boundingBox()
  expect(box!.x).toBeGreaterThanOrEqual(0)
  expect(box!.x + box!.width).toBeLessThanOrEqual(390)
})

test('message GIFs stay still until hovered, focused or tapped; autoplay is opt-in and saved', async ({
  page,
}) => {
  await mockMedia(page)
  await page.getByRole('button', { name: 'Choose a GIF', exact: true }).click()
  await page.getByRole('button', { name: 'Trending', exact: true }).click()
  await page.locator('.gif-select').first().click()
  const frame = page.locator('.message-gif .gif-message-image').last()
  const image = frame.locator('img')
  await expect(frame.locator('.media-kind')).toHaveText('GIF')
  await expect(image).toHaveAttribute('src', /\.jpg$/)
  await frame.hover()
  await expect(image).toHaveAttribute('src', /\.webp$/)
  await page.mouse.move(0, 0)
  await expect(image).toHaveAttribute('src', /\.jpg$/)
  // Keyboard focus plays it too; Enter then opens the viewer.
  await page.keyboard.press('Shift')
  await frame.focus()
  await expect(image).toHaveAttribute('src', /\.webp$/)
  await frame.blur()
  await expect(image).toHaveAttribute('src', /\.jpg$/)

  await page.getByRole('button', { name: 'User settings', exact: true }).click()
  await page.getByRole('button', { name: 'Notifications', exact: true }).click()
  const autoplay = page.getByRole('switch', { name: 'Autoplay GIFs and videos' })
  await expect(autoplay).toHaveAttribute('aria-checked', 'false')
  const saved = page.waitForResponse(
    (r) => r.url().includes('/preferences') && r.request().method() === 'PUT' && r.ok(),
  )
  await autoplay.click()
  await expect(autoplay).toHaveAttribute('aria-checked', 'true')
  await saved
  // Drop the device copy so the reload has to read the setting back from the account.
  await page.evaluate(() => localStorage.removeItem('ircord.preferences'))
  await page.reload()
  await expect(page.locator('.message-gif .gif-message-image img').last()).toHaveAttribute(
    'src',
    /\.webp$/,
  )

  // Reduced motion holds the still frame even with autoplay on, and hover does not play it.
  await page.emulateMedia({ reducedMotion: 'reduce' })
  const still = page.locator('.message-gif .gif-message-image').last()
  await expect(still.locator('img')).toHaveAttribute('src', /\.jpg$/)
  await still.hover()
  await expect(still.locator('img')).toHaveAttribute('src', /\.jpg$/)
})
