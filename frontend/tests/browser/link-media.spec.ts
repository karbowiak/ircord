import { test, expect, type Page } from '@playwright/test'
import { mkdir, readFile } from 'node:fs/promises'
import { createFixtures } from '../../server/utils/fixtures'
import { defaultAppearance } from '../../shared/appearance'
import { emptyLibrary } from '../../shared/library'
import { defaultNotificationSettings } from '../../shared/notifications'
import { defaultPreferences } from '../../shared/preferences'

const reddit = 'https://reddit.com/comments/fixture'
const article = 'https://www.dr.dk'
const eve = 'https://eve-kill.com/kill/139603827'
const restricted = 'https://www.instagram.com/reel/De6MOO8M1rc/'
const token = 'a'.repeat(48)
const fallbackToken = 'b'.repeat(48)
const summaryToken = 'c'.repeat(48)
const eveToken = 'd'.repeat(48)
const githubToken = 'e'.repeat(48)
const responsiveToken = '9'.repeat(48)
const github = 'https://github.com/karbowiak/ircord/issues/5'
const spotify = 'https://open.spotify.com/playlist/37i9dQZF1DX4sWSpwq3LiO'
const youtube = 'https://youtu.be/R2EI1r9uqGQ'
const cnn = 'https://edition.cnn.com/2026/11/08/politics/live-news/midterm-election-trump-congress'
const xPost = 'https://x.com/Osinttechnical/status/2119345734642475100'
const blueskyPost = 'https://bsky.app/profile/chriso-wiki.bsky.social/post/3mxgtsqzqjs2t'
const directGif = 'https://media.example.test/animation.gif'
const directVideo = 'https://media.example.test/movie.mp4'
const directImage = 'https://media.example.test/first.jpg'
const uploadMov = 'https://media.example.test/movie.mov'
const uploadPng = 'https://media.example.test/still.png'
const now = new Date().toISOString()

/** Entirely synthetic account and private history: no signup or IRC traffic. */
async function workspace(page: Page, { failNative = false, providerFailures = false, richProviders = false, galleryErrors = false, longHistory=false, socialProviders=false, directFiles=false, uploads=false, delayArticle } : {failNative?:boolean;providerFailures?:boolean;richProviders?:boolean;galleryErrors?:boolean;longHistory?:boolean;socialProviders?:boolean;directFiles?:boolean;uploads?:boolean;delayArticle?:Promise<void>} = {}) {
  const { bootstrap } = createFixtures()
  const conversation = bootstrap.conversations.find(c => c.id === 'general')!
  bootstrap.conversations = [conversation]
  const items = [
    { id: 'media:native', conversationId: 'general', senderId: 'mika', kind: 'message', text: reddit, createdAt: now, reactions: [] },
    { id: 'media:widget', conversationId: 'general', senderId: 'jules', kind: 'message', text: article, createdAt: now, reactions: [] },
    { id: 'media:code', conversationId: 'general', senderId: 'noah', kind: 'message', text: '\x11https://code.example.test/inline\x11\n```txt\nhttps://code.example.test/block\n```', createdAt: now, reactions: [] },
  ]
  if (providerFailures) items.push(
    { id: 'media:blocked', conversationId: 'general', senderId: 'jules', kind: 'message', text: eve, createdAt: now, reactions: [] },
    { id: 'media:restricted', conversationId: 'general', senderId: 'jules', kind: 'message', text: restricted, createdAt: now, reactions: [] },
  )
  if (richProviders) items.push(
    { id: 'media:github', conversationId: 'general', senderId: 'jules', kind: 'message', text: github, createdAt: now, reactions: [] },
    { id: 'media:spotify', conversationId: 'general', senderId: 'jules', kind: 'message', text: spotify, createdAt: now, reactions: [] },
    { id: 'media:youtube', conversationId: 'general', senderId: 'jules', kind: 'message', text: youtube, createdAt: now, reactions: [] },
    ...(process.env.TEST_RESPONSIVE_WIDGET_DOCUMENT ? [{ id: 'media:cnn', conversationId: 'general', senderId: 'jules', kind: 'message', text: cnn, createdAt: now, reactions: [] }] : []),
  )
  const resolutions: { url: string; widget?: boolean }[] = []
  if (socialProviders) items.splice(0, items.length, ...[xPost, blueskyPost].map((text, index) => ({ id: `social:${index}`, conversationId: 'general', senderId: 'jules', kind: 'message', text, createdAt: now, reactions: [] })))
  if (directFiles) items.splice(0, items.length, ...[directGif, directVideo, directImage].map((text, index) => ({ id: `file:${index}`, conversationId: 'general', senderId: 'jules', kind: 'message', text, createdAt: now, reactions: [] })))
  if (uploads) items.splice(0, items.length, ...[
    { name: 'animation.gif', url: directGif, contentType: 'image/gif', width: process.env.TEST_LINK_GIF_CAPTURE ? 450 : 320, height: process.env.TEST_LINK_GIF_CAPTURE ? 338 : 200 },
    { name: 'recording.mov', url: uploadMov, contentType: 'application/octet-stream', width: 320, height: 200 },
    { name: 'capture.png', url: uploadPng, contentType: 'image/png', width: 320, height: 200 },
    { name: 'photo.jpg', url: directImage, contentType: 'image/jpeg', width: 960, height: 640 },
    { name: 'notes.pdf', url: 'https://media.example.test/notes.pdf', contentType: 'application/pdf', width: 0, height: 0 },
  ].map((attachment, index) => ({ id: `upload:${index}`, conversationId: 'general', senderId: 'jules', kind: 'message', text: attachment.url, createdAt: now, reactions: [], attachment: { ...attachment, alt: '', size: 40000 } })))
  if(longHistory) items.splice(0,items.length,...Array.from({length:80},(_,index)=>({id:`anchor:${index}`,conversationId:'general',senderId:index%2?'mika':'jules',kind:'message',text:index===40?article:`Reading anchor ${index}`,createdAt:now,reactions:[]})))
  await page.routeWebSocket('**/api/v1/events*', socket => {
    socket.send(JSON.stringify({ v: 1, type: 'session.ready', cursor: 0, emoteVersion: 0 }))
  })
  await page.route('https://media.example.test/**', route => {
    if (route.request().url() === directGif) return route.fulfill({ path: process.env.TEST_LINK_GIF_CAPTURE || 'tests/fixtures/link-motion.gif', contentType: 'image/gif' })
    if (route.request().url() === directVideo) return route.fulfill({ path: 'tests/fixtures/link-motion.mp4', contentType: 'video/mp4' })
    if (route.request().url() === uploadMov) return route.fulfill({ path: 'tests/fixtures/link-motion.mov', contentType: 'video/quicktime' })
    if (route.request().url() === uploadPng) return route.fulfill({ path: 'tests/fixtures/link-still.png', contentType: 'image/png' })
    if (failNative || route.request().url().endsWith('missing.jpg')) return route.abort()
    if (galleryErrors && route.request().url().endsWith('second.jpg')) return route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="200"><rect width="1200" height="200" fill="#aabfd9"/></svg>'})
    return route.fulfill({ path: 'public/media/weekend.jpg', contentType: 'image/jpeg' })
  })
  await page.route(/^https?:\/\/[^/]+\/api\//, async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (path.startsWith('/api/v1/link-media/widgets/')) {
      const currentToken = path.split('/').at(-1)
      const document = currentToken === summaryToken ? process.env.TEST_IFRAMELY_WIDGET_DOCUMENT : currentToken === eveToken ? process.env.TEST_IFRAMELY_EVE_DOCUMENT : currentToken === githubToken ? process.env.TEST_IFRAMELY_GITHUB_DOCUMENT : undefined
      if (document) return route.fulfill({
        contentType: 'text/html',
        headers: { 'Referrer-Policy':'strict-origin-when-cross-origin', 'Content-Security-Policy': "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src https:; frame-src https:; sandbox allow-scripts; frame-ancestors 'self'" },
        body: (await readFile(document, 'utf8')).replace('color-scheme:light dark',`color-scheme:${new URL(request.url()).searchParams.get('theme')==='light'?'light':'dark'}`),
      })
      return route.fulfill({
        contentType: 'text/html',
        headers: { 'X-Frame-Options': 'SAMEORIGIN', 'Content-Security-Policy': "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; sandbox allow-scripts; frame-ancestors 'self'" },
        body: `<!doctype html><html><body style="margin:0;padding:20px;background:#1d1d23;color:#eeeef2;font:14px system-ui"><strong>Iframely widget fixture</strong><p>This document stays isolated from the chat.</p><script>try {parent.document.body.dataset.widgetEscaped='yes'} catch {} parent.postMessage({type:'ircord:widget-size',token:'${currentToken}',height:160},'*')</script></body></html>`,
      })
    }
    let data: unknown = {}
    if (path === '/api/v1/auth/session') data = { account: { id: 'media-fixture', username: 'media-fixture', createdAt: now } }
    else if (path === '/api/v1/auth/registration') data = { enabled: false, requireKey: false }
    else if (path === '/api/v1/bootstrap') data = bootstrap
    else if (path.endsWith('/link-media')) {
      const input = request.postDataJSON() as { url: string; widget?: boolean }
      resolutions.push(input)
      if (directFiles) return route.fulfill({ json: { url: input.url, provider: 'media', assets: [{ url: input.url, kind: input.url === directVideo ? 'video' : 'image', contentType: input.url === directGif ? 'image/gif' : input.url === directVideo ? 'video/mp4' : 'image/jpeg', width: input.url === directImage ? 960 : input.url === directGif && process.env.TEST_LINK_GIF_CAPTURE ? 450 : 320, height: input.url === directImage ? 640 : input.url === directGif && process.env.TEST_LINK_GIF_CAPTURE ? 338 : 200 }] } })
      if (socialProviders) return route.fulfill({ json: { url: input.url, provider: 'iframely', title: input.url === xPost ? 'X post' : 'Bluesky post', assets: [], widget: JSON.parse(await readFile((input.url === xPost ? process.env.TEST_X_WIDGET_CAPTURE : process.env.TEST_BLUESKY_WIDGET_CAPTURE)!, 'utf8')) } })
      if(input.url===article && delayArticle) await delayArticle
      if (richProviders && input.url === github) return route.fulfill({json:{url:github,provider:'iframely',title:'GitHub issue',assets:[],widget:{url:`/api/v1/link-media/widgets/${githubToken}`,external:false,token:githubToken,height:350}}})
      if (richProviders && input.url === spotify) return route.fulfill({json:{url:spotify,provider:'iframely',title:'Spotify playlist',assets:[],widget:JSON.parse(await readFile(process.env.TEST_SPOTIFY_WIDGET_DOCUMENT!, 'utf8'))}})
      if (richProviders && input.url === youtube) return route.fulfill({json:{url:youtube,provider:'iframely',title:'YouTube video',assets:[],widget:process.env.TEST_YOUTUBE_WIDGET_DOCUMENT ? JSON.parse(await readFile(process.env.TEST_YOUTUBE_WIDGET_DOCUMENT,'utf8')) : {url:'https://www.youtube.com/embed/R2EI1r9uqGQ',external:true,token:'f'.repeat(48),height:270}}})
      if (richProviders && input.url === cnn) return route.fulfill({json:{url:cnn,provider:'iframely',title:'CNN video',assets:[],widget:JSON.parse(await readFile(process.env.TEST_RESPONSIVE_WIDGET_DOCUMENT!,'utf8'))}})
      if (providerFailures && input.url === eve && process.env.TEST_IFRAMELY_EVE_DOCUMENT) return route.fulfill({json:{url:eve,provider:'iframely',title:'EVE-KILL',assets:[],widget:{url:`/api/v1/link-media/widgets/${eveToken}`,external:false,token:eveToken,height:250}}})
      if (providerFailures && [eve, restricted].includes(input.url)) return route.fulfill({status:422,json:{title:'Unprocessable Entity',detail:input.url===eve ? 'This site refused the preview request. Open the original to view it.' : 'This post isn’t publicly accessible. It may require signing in or age verification on the original site.'}})
      if (providerFailures && input.url === article) return route.fulfill({json:{url:article,provider:'iframely',title:'DR: Nyheder',assets:[],widget:{url:`/api/v1/link-media/widgets/${summaryToken}`,external:false,token:summaryToken,height:350}}})
      data = input.url === reddit && !input.widget ? {
        url: reddit, provider: 'reddit', title: 'A quiet weekend by the water', author: 'fixture-photographer',
        description: 'Synthetic post for the client test.\nThe first stop.\nThe second stop.\nThe third stop.\nThe fourth stop.\nThe final stop.',
        assets: [
          { url: 'https://media.example.test/first.jpg', kind: 'image', width: 1200, height: 800 },
          ...(!failNative ? [{ url: 'https://media.example.test/second.jpg', kind: 'image', width: 1200, height: galleryErrors ? 200 : 800 }] : []),
          ...(galleryErrors ? [{ url: 'https://media.example.test/missing.jpg', kind: 'image', width: 0, height: 0 }] : []),
        ],
      } : { url: input.url, provider: 'iframely', title: input.url === reddit ? 'Fallback widget' : 'Article widget', assets: [], widget: { url: `/api/v1/link-media/widgets/${input.widget ? fallbackToken : token}`, external: false, token: input.widget ? fallbackToken : token, height: 220 } }
    } else if (path === '/api/v1/links') data = {
      items: items.slice(0, 2).map(item => ({ identity: item.text, url: item.text, host: new URL(item.text).host, kind: 'link', count: 1, lastPostedAt: now, sources: [{ messageId: item.id, conversationId: 'general', networkId: conversation.networkId, senderId: item.senderId, postedAt: now, url: item.text, excerpt: item.text }] })),
      nextCursor: null, indexing: false,
    }
    else if (path === '/api/v1/conversations/general/messages') data = { items, hasMore: false, hasNewer: false, nextCursor: null }
    else if (path === '/api/v1/notifications/settings') data = defaultNotificationSettings
    else if (path === '/api/v1/emotes') data = { version: 0, emotes: [], canManage: false }
    else if (path === '/api/v1/saved-message-ids' || path.includes('/inbox')) data = { items: [], nextCursor: null, total: 0, unread: 0 }
    else if (path === '/api/app/appearance') data = { version: 1, settings: defaultAppearance, updatedAt: null }
    else if (path.startsWith('/api/app/library')) data = emptyLibrary()
    else if (path === '/api/app/preferences') data = { version: 1, preferences: defaultPreferences, updatedAt: null }
    return route.fulfill({ json: data })
  })
  await page.goto('/')
  await expect(page.getByRole('combobox', { name: 'Message #general' })).toBeVisible()
  return resolutions
}

test('native galleries and isolated widgets work on desktop and mobile', async ({ page }) => {
  const resolutions = await workspace(page)
  const native = page.getByRole('region', { name: 'Reddit post' })
  await expect(native.getByRole('link', { name: 'A quiet weekend by the water' })).toBeVisible()
  const photo = native.locator('.native-media-slide.is-current .native-media-photo')
  await expect(photo).toHaveJSProperty('naturalWidth', 960)
  await native.getByRole('button', { name: 'Next media' }).click()
  await expect(photo).toHaveAttribute('src', /second\.jpg$/)
  await expect(native.getByRole('status')).toHaveText('2 / 2')
  await native.getByRole('button', { name: 'Open image 2 of 2' }).click()
  await expect(page.getByRole('dialog', { name: 'A quiet weekend by the water', exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog', { name: 'A quiet weekend by the water', exact: true })).toHaveCount(0)
  const frame = page.locator('.link-widget iframe').first()
  await frame.scrollIntoViewIfNeeded()
  await expect(frame).toHaveAttribute('height', '160')
  await expect(frame).not.toHaveAttribute('sandbox', /allow-same-origin/)
  expect(await page.locator('body').getAttribute('data-widget-escaped')).toBeNull()
  expect(resolutions.some(r => r.url.includes('code.example.test'))).toBe(false)
  await native.scrollIntoViewIfNeeded()
  await expect(native.getByRole('button', { name: 'Read more', exact: true })).toBeVisible()
  await mkdir('.impeccable/review', { recursive: true })
  await page.screenshot({ path: '.impeccable/review/desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await native.scrollIntoViewIfNeeded()
  await native.getByRole('button', { name: 'Read more', exact: true }).click()
  await expect(native.locator('.native-media-caption')).toContainText('The final stop.')
  await expect(native.getByRole('button', { name: 'Show less', exact: true })).toHaveAttribute('aria-expanded', 'true')
  const caption = native.locator('.native-media-caption')
  expect(await caption.evaluate(el => el.scrollHeight <= el.clientHeight + 1)).toBe(true)
  await native.getByRole('button', { name: 'Show less', exact: true }).click()
  await expect(native.getByRole('button', { name: 'Previous media' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: '.impeccable/review/mobile.png' })
})

test('a broken native source falls back to the Iframely widget', async ({ page }) => {
  const resolutions = await workspace(page, { failNative: true })
  await expect(page.locator(`iframe[src*="${fallbackToken}"]`)).toBeVisible()
  expect(resolutions).toContainEqual({ url: reddit, widget: true })
  await expect(page.locator('.native-link-media')).toHaveCount(0)
})

test('the upstream Links view opens without retired preview settings', async ({ page }) => {
  await workspace(page)
  await page.getByRole('button', { name: 'Links in #general' }).click()
  const dialog = page.getByRole('dialog', { name: 'Links', exact: true })
  await expect(dialog.locator('.catalog-card')).toHaveCount(2)
  await expect(dialog.locator('.catalog-card-title').first()).toHaveText('reddit.com/comments/fixture')
  await expect(dialog.locator('img')).toHaveCount(0)
  await page.screenshot({ path: '.impeccable/review/links-desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(dialog.getByRole('textbox', { name: 'Filter links' })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: '.impeccable/review/links-mobile.png' })
})

test('Iframely metadata cards and provider errors stay readable', async ({ page }) => {
  test.skip(!process.env.TEST_IFRAMELY_WIDGET_DOCUMENT, 'Use the opt-in Go capture of the real Iframely response')
  const resolutions=await workspace(page, { providerFailures: true })
  const widget = page.locator(`iframe[src*="${summaryToken}"]`)
  await widget.scrollIntoViewIfNeeded()
  const summary = page.frameLocator(`iframe[src*="${summaryToken}"]`).locator('.summary')
  await expect(summary).toContainText('DR: Nyheder')
  await expect(summary.locator('img')).toBeVisible()
  if (process.env.TEST_IFRAMELY_EVE_DOCUMENT) {
    const eveSummary = page.frameLocator(`iframe[src*="${eveToken}"]`).locator('.summary')
    await expect(eveSummary).toContainText('Anhinga | Auroraqual')
    await expect(eveSummary.locator('img')).toHaveJSProperty('naturalWidth', 64)
    const image = await eveSummary.locator('img').boundingBox()
    const text = await eveSummary.locator('div').boundingBox()
    expect(image!.width).toBe(64)
    expect(image!.x).toBeGreaterThan(text!.x)
  } else await expect(page.getByText('This site refused the preview request. Open the original to view it.', { exact: true })).toBeVisible()
  await expect(page.getByText(/This post isn’t publicly accessible/)).toBeVisible()
  const failure=page.locator('.message-link').filter({hasText:'This post isn’t publicly accessible'})
  const retry=failure.getByRole('button',{name:'Retry',exact:true})
  expect((await retry.boundingBox())!.height).toBeGreaterThanOrEqual(36)
  await retry.click()
  await expect(failure.getByRole('button',{name:'Retrying…'})).toBeDisabled()
  await page.screenshot({path:'.impeccable/review/provider-retry-desktop.png'})
  await expect(failure.getByRole('button',{name:'Retry',exact:true})).toBeEnabled()
  expect(resolutions.filter(item=>item.url===restricted)).toHaveLength(2)
  await expect(page.getByRole('link', { name: 'Open original', exact: true })).toHaveCount(0)
  await page.screenshot({ path: '.impeccable/review/provider-desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await widget.scrollIntoViewIfNeeded()
  await expect(summary).toContainText('DR: Nyheder')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: '.impeccable/review/provider-mobile.png' })
})

test('gallery height stays steady through landscape and missing slides', async ({ page }) => {
  await workspace(page, { galleryErrors: true })
  const native=page.getByRole('region',{name:'Reddit post'})
  const stage=native.locator('.native-media-stage')
  await native.scrollIntoViewIfNeeded()
  const start=await stage.boundingBox()
  await native.getByRole('button',{name:'Next media'}).click()
  await expect(native.locator('.native-media-slide.is-current .native-media-photo')).toHaveJSProperty('naturalHeight',200)
  expect((await stage.boundingBox())!.height).toBe(start!.height)
  await native.getByRole('button',{name:'Next media'}).click()
  await expect(native.getByText('This media couldn’t be loaded.')).toBeVisible()
  expect((await stage.boundingBox())!.height).toBe(start!.height)
  await page.screenshot({path:'.impeccable/review/gallery-error-desktop.png'})
  await native.getByRole('button',{name:'Retry',exact:true}).click()
  await expect(native.getByText('Retrying media…',{exact:true})).toBeVisible()
  await page.screenshot({path:'.impeccable/review/gallery-retry-desktop.png'})
  await expect(native.getByText('This media couldn’t be loaded.')).toBeVisible()
  await page.setViewportSize({width:390,height:844})
  const mobile=await stage.boundingBox()
  await expect(native.getByText('This media couldn’t be loaded.')).toBeVisible()
  await page.screenshot({path:'.impeccable/review/gallery-error-mobile.png'})
  await native.getByRole('button',{name:'Previous media'}).click()
  expect((await stage.boundingBox())!.height).toBe(mobile!.height)
})

test('GitHub context survives image failure and external players receive origin and height', async ({ page }) => {
  test.skip(!process.env.TEST_IFRAMELY_GITHUB_DOCUMENT || !process.env.TEST_SPOTIFY_WIDGET_DOCUMENT,'Use opt-in real Iframely captures')
  await page.route('https://opengraph.githubassets.com/**',route=>route.abort())
  const cnnRequests: string[]=[]
  await page.route('https://fave.api.cnn.io/**',async route=>{
    cnnRequests.push((await route.request().allHeaders()).referer)
    return route.fulfill({contentType:'text/html',body:'<body style="margin:0;background:#111;color:#fff;display:grid;place-content:center;height:100vh;font:16px system-ui">Responsive player fixture<script>try {void document.cookie;void window.localStorage;document.body.dataset.storage="available"} catch {document.body.dataset.storage="blocked"}try{parent.document.body.dataset.playerEscaped="yes"}catch{}</script></body>'})
  })
  const youtubeRequest=page.waitForRequest(request=>request.url().startsWith('https://www.youtube.com/embed/'))
  const playerErrors:string[]=[]
  page.on('pageerror',error=>{if(/forbidden.*sandbox|sandbox.*allow-same-origin/i.test(error.message)) playerErrors.push(error.message)})
  if(!process.env.TEST_REAL_YOUTUBE) await page.route('https://www.youtube.com/embed/**',route=>route.fulfill({contentType:'text/html',body:'<body style="margin:0;background:#111;color:#fff;font:16px system-ui">YouTube request fixture</body>'}))
  await workspace(page,{richProviders:true})
  await page.getByRole('link',{name:youtube,exact:true}).scrollIntoViewIfNeeded()
  const request=await youtubeRequest
  expect((await request.allHeaders()).referer).toBe(new URL(page.url()).origin+'/')
  await expect(page.locator('iframe[title="YouTube video"]')).toHaveAttribute('sandbox',/allow-same-origin/)
  if(process.env.TEST_REAL_YOUTUBE) {
    const video=page.frameLocator('iframe[title="YouTube video"]')
    await expect(video.locator('.html5-video-player')).toBeVisible({timeout:15000})
    await expect(video.getByText('Video player configuration error',{exact:true})).toHaveCount(0)
    expect(playerErrors).toEqual([])
  }
  const spotifyFrame=page.locator('iframe[title="Spotify playlist"]')
  await page.getByRole('link',{name:spotify,exact:true}).scrollIntoViewIfNeeded()
  await expect(spotifyFrame).toHaveAttribute('height','352')
  await expect(spotifyFrame).toHaveAttribute('referrerpolicy','strict-origin-when-cross-origin')
  if(process.env.TEST_RESPONSIVE_WIDGET_DOCUMENT) {
    await page.getByRole('link',{name:cnn,exact:true}).scrollIntoViewIfNeeded()
    await expect(page.getByRole('button',{name:'Load video player'})).toBeVisible()
    expect(cnnRequests).toHaveLength(0)
    await page.screenshot({path:'.impeccable/review/responsive-poster-desktop.png'})
  }
  const card=page.frameLocator(`iframe[src*="${githubToken}"]`).locator('.github-card')
  await page.getByRole('link',{name:github,exact:true}).scrollIntoViewIfNeeded()
  await expect(card.locator('.github-repository')).toHaveText('karbowiak/ircord')
  await expect(card.locator('.github-reference')).toHaveText('Issue #5')
  await expect(card.locator('h2')).toHaveText('Add message editing with an agreed interoperable protocol')
  await expect(card.locator('img')).toHaveCount(0)
  await expect(page.getByRole('link',{name:'Open original',exact:true})).toHaveCount(0)
  await page.screenshot({path:'.impeccable/review/github-desktop.png'})
  await page.setViewportSize({width:390,height:844})
  await page.locator(`iframe[src*="${githubToken}"]`).scrollIntoViewIfNeeded()
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true)
  await page.screenshot({path:'.impeccable/review/github-mobile.png'})
  if (process.env.TEST_RESPONSIVE_WIDGET_DOCUMENT) {
    await page.getByRole('link',{name:cnn,exact:true}).scrollIntoViewIfNeeded()
    await expect(page.getByRole('button',{name:'Load video player'})).toBeVisible()
    expect(cnnRequests).toHaveLength(0)
    await page.screenshot({path:'.impeccable/review/responsive-poster-mobile.png'})
    await page.getByRole('button',{name:'Load video player'}).click()
    const player=page.locator('iframe[title="CNN video"]')
    await expect(player).toHaveAttribute('sandbox',/allow-same-origin/)
    await expect(page.frameLocator('iframe[title="CNN video"]').locator('body')).toHaveAttribute('data-storage','available')
    expect(await player.evaluate(element=>Math.round(element.getBoundingClientRect().height))).toBe(180)
    expect(cnnRequests[0]).toBe(new URL(page.url()).origin+'/')
    expect(await page.locator('body').getAttribute('data-player-escaped')).toBeNull()
    await page.screenshot({path:'.impeccable/review/responsive-mobile.png'})
    await page.setViewportSize({width:1440,height:900})
    await player.scrollIntoViewIfNeeded()
    expect(await player.evaluate(element=>Math.round(element.getBoundingClientRect().height))).toBe(270)
    await page.screenshot({path:'.impeccable/review/responsive-desktop.png'})
  }
  await page.getByRole('link',{name:github,exact:true}).scrollIntoViewIfNeeded()
  await expect(card.locator('.github-copy')).toBeVisible()
  const dark=await card.evaluate(element=>getComputedStyle(element).backgroundColor)
  await page.evaluate(settings=>{
    localStorage.setItem('ircord.appearance',JSON.stringify({settings:{...settings,mode:'light'},sync:false,revision:null,dirty:false}))
    dispatchEvent(new StorageEvent('storage',{key:'ircord.appearance'}))
  },defaultAppearance)
  await expect(page.locator('html')).toHaveAttribute('data-theme','light')
  await expect.poll(()=>card.evaluate(element=>getComputedStyle(element).backgroundColor)).not.toBe(dark)
  await page.screenshot({path:'.impeccable/review/github-light-desktop.png'})
  await page.setViewportSize({width:390,height:844})
  await page.getByRole('link',{name:github,exact:true}).scrollIntoViewIfNeeded()
  await page.screenshot({path:'.impeccable/review/github-light-mobile.png'})
})

test('late embeds preserve the reading anchor while scrolling upward', async ({page})=>{
  let release!:()=>void
  const pending=new Promise<void>(resolve=>release=resolve)
  const resolutions=await workspace(page,{longHistory:true,delayArticle:pending})
  const pane=page.locator('.timeline-scroll')
  await pane.evaluate(element=>element.scrollTop=element.scrollHeight*0.43)
  await expect.poll(()=>resolutions.some(item=>item.url===article)).toBe(true)
  const loading=page.locator('.timeline-row[data-index="40"]')
  const position=await loading.evaluate(element=>{
    const pane=element.closest('.timeline-scroll')!
    return element.getBoundingClientRect().bottom-pane.getBoundingClientRect().top+pane.scrollTop+10
  })
  await pane.evaluate((element,top)=>element.scrollTop=top,position)
  const anchor=page.getByText('Reading anchor 43',{exact:true})
  await expect(anchor).toBeVisible()
  const before=(await anchor.boundingBox())!.y
  release()
  await expect(page.locator('.link-widget iframe')).toHaveAttribute('height','160')
  await expect.poll(async()=>Math.abs((await anchor.boundingBox())!.y-before)).toBeLessThanOrEqual(2)
})

test('real X and Bluesky Iframely widgets render media and size themselves', async ({ page }) => {
  test.skip(!process.env.TEST_X_WIDGET_CAPTURE || !process.env.TEST_BLUESKY_WIDGET_CAPTURE, 'Opt-in real Iframely social widgets')
  await workspace(page, { socialProviders: true })
  await mkdir('.impeccable/review', { recursive: true })
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport)
    for (const [title, host, text, imageHost] of [
      ['X post', 'platform.twitter.com', 'Budgetary issues', 'pbs.twimg.com/media'],
      ['Bluesky post', 'embed.bsky.app', 'Russian attacks on Ukrainian data centres', 'cdn.bsky.app/img'],
    ]) {
      const frame = page.locator(`iframe[title="${title}"]`)
      await frame.scrollIntoViewIfNeeded()
      await expect(frame).toHaveAttribute('src', new RegExp(`https://${host}/`))
      await expect(frame).toHaveAttribute('sandbox', /allow-same-origin/)
      const content = page.frameLocator(`iframe[title="${title}"]`)
      await expect(content.locator('body')).toContainText(text!)
      await expect(content.locator(`img[src*="${imageHost}"]`).first()).toBeVisible()
      await expect.poll(() => frame.getAttribute('height')).not.toBe('600')
      if (title === 'X post') await expect.poll(async () => Math.abs(Number(await frame.getAttribute('height')) - await content.locator('#app').evaluate(element => element.getBoundingClientRect().height))).toBeLessThanOrEqual(2)
      expect(await frame.evaluate(element => element.getBoundingClientRect().width)).toBeLessThanOrEqual(480)
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
      await page.screenshot({ path: `.impeccable/review/${title!.startsWith('X') ? 'x' : 'bluesky'}-${viewport.width > 700 ? 'desktop' : 'mobile'}.png` })
    }
  }
})

test('direct GIF and video previews wait for hover, keyboard or touch', async ({ page, browser }) => {
  await workspace(page, { directFiles: true })
  await page.mouse.move(0, 0)
  const gif = page.getByLabel('GIF preview', { exact: true }).filter({ has: page.locator('button') })
  await gif.scrollIntoViewIfNeeded()
  await expect(gif.locator('canvas')).toBeVisible()
  await expect(gif.locator('img')).toBeHidden()
  const still = await gif.screenshot()
  await page.waitForTimeout(250)
  expect((await gif.screenshot()).equals(still)).toBe(true)
  await page.screenshot({ path: '.impeccable/review/direct-media-desktop.png' })
  await gif.hover()
  await expect(gif.locator('img')).toBeVisible()
  await expect(gif.locator('canvas')).toBeHidden()
  await page.mouse.move(0, 0)
  await expect(gif.locator('canvas')).toBeVisible()
  const player = page.locator('.direct-link-media video')
  await player.scrollIntoViewIfNeeded()
  await expect(player).toHaveJSProperty('paused', true)
  await expect(player).toHaveJSProperty('currentTime', 0)
  await player.hover()
  await expect(player).toHaveJSProperty('paused', false)
  await expect(player).toHaveJSProperty('muted', true)
  await expect.poll(() => player.evaluate(element => (element as HTMLVideoElement).currentTime)).toBeGreaterThan(0)
  await page.mouse.move(0, 0)
  await expect(player).toHaveJSProperty('paused', true)
  await gif.scrollIntoViewIfNeeded()
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await gif.hover()
  await expect(gif.locator('canvas')).toBeVisible()
  await gif.getByRole('button').focus()
  await page.keyboard.press('Enter')
  await expect(gif.locator('img')).toBeVisible()
  const mobile = await browser.newContext({ baseURL: process.env.TEST_BASE_URL || 'http://127.0.0.1:4173', viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true })
  const touch = await mobile.newPage()
  await workspace(touch, { directFiles: true })
  const mobileGif = touch.locator('.direct-link-media').filter({ has: touch.locator('canvas') })
  await mobileGif.scrollIntoViewIfNeeded()
  await expect(mobileGif.locator('canvas')).toBeVisible()
  await touch.screenshot({ path: '.impeccable/review/direct-media-mobile.png' })
  await mobileGif.getByRole('button').tap()
  await expect(mobileGif.locator('img')).toBeVisible()
  await mobileGif.getByRole('button').tap()
  await expect(mobileGif.locator('canvas')).toBeVisible()
  await mobile.close()
})

test('uploaded media previews retain downloads and handle unsupported video', async ({ page }) => {
  const resolutions = await workspace(page, { uploads: true })
  await page.mouse.move(0, 0)
  const attachment = (name: string) => page.locator('.message-attachment').filter({ has: page.getByText(name, { exact: true }) })
  const gif = attachment('animation.gif')
  await gif.scrollIntoViewIfNeeded()
  await expect(gif.locator('canvas')).toBeVisible()
  await expect(gif.locator('img')).toBeHidden()
  const movie = attachment('recording.mov')
  const player = movie.locator('video')
  await player.scrollIntoViewIfNeeded()
  await expect(player).toHaveJSProperty('paused', true)
  await player.hover()
  await expect(player).toHaveJSProperty('paused', false)
  await page.mouse.move(0, 0)
  await expect(player).toHaveJSProperty('paused', true)
  await page.screenshot({ path: '.impeccable/review/uploads-desktop.png' })
  for (const name of ['capture.png', 'photo.jpg']) {
    const image = attachment(name)
    await image.scrollIntoViewIfNeeded()
    await expect(image.getByRole('img', { name })).toBeVisible()
    await expect(image.locator('.direct-media-label')).toHaveText('IMAGE')
  }
  await expect(attachment('notes.pdf').locator('.direct-link-media')).toHaveCount(0)
  await player.dispatchEvent('error')
  await expect(movie.getByText('Preview unavailable. Download the file to view it.')).toBeVisible()
  await expect(movie.locator('a[download="recording.mov"]')).toHaveAttribute('href', uploadMov)
  expect(resolutions).toHaveLength(0)
  await page.setViewportSize({ width: 390, height: 844 })
  await gif.scrollIntoViewIfNeeded()
  await page.screenshot({ path: '.impeccable/review/uploads-mobile.png' })
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
})
