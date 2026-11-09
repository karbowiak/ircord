// Real Chromium, IndexedDB worker, authenticated Go API, and durable disposable
// history. No production services or accounts are contacted.
import assert from 'node:assert/strict'
import { spawn } from 'node:child_process'
import { mkdir } from 'node:fs/promises'
import { chromium } from 'playwright'
import { verifyOfflineWorker } from './verify-offline-worker.mjs'

const { api, ui, uiPort, owner, username, password, control } = JSON.parse(
  process.env.IRCORD_SYNC_TEST,
)
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))
async function until(check, label) {
  for (let i = 0; i < 300; i++) {
    try {
      if (await check()) return
    } catch (error) {
      if (!/Execution context was destroyed/.test(error.message)) throw error
    }
    await delay(40)
  }
  throw new Error(`Timed out: ${label}`)
}
const frontend = spawn(process.execPath, ['.output/server/index.mjs'], {
  env: {
    ...process.env,
    HOST: '127.0.0.1',
    PORT: String(uiPort),
    NUXT_DEMO: 'false',
    NUXT_BACKEND_URL: api,
    NUXT_PUBLIC_API_BASE: '/api/v1',
    NUXT_PUBLIC_NETWORK_PRESETS: '[]',
  },
  stdio: ['ignore', 'pipe', 'pipe'],
})
let log = ''
frontend.stdout.on('data', (chunk) => {
  log = (log + chunk).slice(-2500)
})
frontend.stderr.on('data', (chunk) => {
  log = (log + chunk).slice(-2500)
})
let browser
let page
const errors = []
async function archive(page) {
  return page.evaluate(async (owner) => {
    if (!(await indexedDB.databases()).some((db) => db.name === `ircord-offline-${owner}`))
      return null
    const db = await new Promise((resolve, reject) => {
      const q = indexedDB.open(`ircord-offline-${owner}`)
      q.onsuccess = () => resolve(q.result)
      q.onerror = () => reject(q.error)
    })
    try {
      if (!db.objectStoreNames.contains('meta')) return null
      const tx = db.transaction(['meta', 'messages', 'files'])
      const read = (q) =>
        new Promise((resolve, reject) => {
          q.onsuccess = () => resolve(q.result)
          q.onerror = () => reject(q.error)
        })
      const [meta, messages, files] = await Promise.all([
        read(tx.objectStore('meta').get('archive')),
        read(tx.objectStore('messages').getAll()),
        read(tx.objectStore('files').getAll()),
      ])
      return {
        meta,
        messages: messages.map((row) => ({
          id: row.id,
          text: row.message.text,
          deleted: row.message.deleted,
          pinned: row.pinned,
          seq: row.seq,
          conversationId: row.conversationId,
        })),
        files: files.map((file) => ({
          url: file.url,
          size: file.blob?.size ?? 0,
          refs: file.refs,
        })),
      }
    } finally {
      db.close()
    }
  }, owner)
}
try {
  await until(async () => {
    try {
      return (await fetch(ui)).ok
    } catch {
      return false
    }
  }, 'frontend startup')
  browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    serviceWorkers: 'allow',
    viewport: { width: 1440, height: 900 },
  })
  assert.equal(
    (
      await context.request.post(`${ui}/api/v1/auth/login`, { data: { username, password } })
    ).status(),
    200,
  )
  await context.addInitScript(
    ({ owner }) => {
      sessionStorage.setItem(`ircord.${owner}.active`, JSON.stringify('sync-room-0'))
      Object.defineProperty(document, 'hasFocus', { value: () => false })
      window.__offlineSockets = []
      const Native = WebSocket
      window.WebSocket = class extends Native {
        constructor(...args) {
          super(...args)
          window.__offlineSockets.push(this)
        }
      }
    },
    { owner },
  )
  page = await context.newPage()
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(ui)
  await page.locator('.sidebar-connection').waitFor()
  await until(async () => {
    const value = await archive(page)
    return (
      value?.meta?.complete &&
      value.messages.length === 601 &&
      value.files.some((file) => file.size > 0)
    )
  }, 'seven-day archive and photo bytes')
  let stored = await archive(page)
  assert(!stored.messages.some((row) => row.id === 'outside-retention' || row.id === 'old-photo'))
  assert.equal(stored.meta.state.conversations.length, 4)
  await verifyOfflineWorker(page, page.workers()[0].url(), stored.meta.state)
  console.log(
    'Worker regression checks pass for snapshot races, event gaps, Unicode cursors, pins, and shared-file cleanup.',
  )
  console.log(
    'Background export paged 601 messages into IndexedDB, indexed text, and downloaded the original image.',
  )

  await page.evaluate(async () => {
    await navigator.serviceWorker.ready
    if (!navigator.serviceWorker.controller)
      await new Promise((resolve) =>
        navigator.serviceWorker.addEventListener('controllerchange', resolve, { once: true }),
      )
  })
  // Cache the app shell and worker asset under the controlling service worker.
  await page.reload()
  await page.locator('.sidebar-connection').waitFor()

  const peer = await context.newPage()
  peer.on('pageerror', (error) => errors.push(error.message))
  await peer.goto(ui)
  await peer.locator('.sidebar-connection').waitFor()
  await peer.locator('.conversation-link').filter({ hasText: 'sync-member-0' }).last().click()
  await peer.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .openAround('sync-dm', 'old-photo'),
  )

  // Pin an older photo using real online history, so it survives rolling retention.
  await page.locator('.conversation-link').filter({ hasText: 'sync-member-0' }).last().click()
  await page.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .openAround('sync-dm', 'old-photo'),
  )
  const oldRow = page.locator('.message-row').filter({ hasText: 'Photo kept from last month' })
  await oldRow.getByRole('button', { name: 'Keep offline', exact: true }).click()
  await oldRow.getByText('Kept offline', { exact: true }).waitFor()
  await peer
    .locator('.message-row')
    .filter({ hasText: 'Photo kept from last month' })
    .getByText('Kept offline', { exact: true })
    .waitFor()
  await until(
    async () => (await archive(page)).messages.some((row) => row.id === 'old-photo' && row.pinned),
    'old photo pin',
  )

  await context.setOffline(true)
  await page.evaluate(() => {
    for (const socket of window.__offlineSockets) socket.close()
  })
  await page.locator('.sidebar-connection').getByText('Offline', { exact: true }).waitFor()
  await page.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .saveCheckpoint(['sync-dm', 'sync-room-0']),
  )
  await page.reload()
  await page.locator('.sidebar-connection').getByText('Offline', { exact: true }).waitFor()
  await page
    .locator('.conversation-sidebar')
    .getByRole('button', { name: 'Search messages', exact: true })
    .click()
  await page.getByRole('combobox', { name: 'Search workspace' }).fill('Fixture message')
  await page.getByRole('tab', { name: /^Messages/ }).click()
  await until(
    async () => (await page.locator('.search-message').count()) === 50,
    'offline search first page',
  )
  await page
    .locator('.offline-search-notice')
    .getByText(/Searching saved messages/)
    .waitFor()
  await page.getByRole('button', { name: 'Load older messages', exact: true }).click()
  await until(
    async () => (await page.locator('.search-message').count()) === 100,
    'offline search pagination',
  )
  await page.getByRole('combobox', { name: 'Search workspace' }).fill('東京')
  await until(
    async () => (await page.locator('.search-message').count()) === 50,
    'Unicode offline search',
  )
  await page.getByRole('button', { name: 'Load older messages', exact: true }).click()
  await until(
    async () => (await page.locator('.search-message').count()) === 100,
    'Unicode search pagination',
  )
  await page.getByRole('combobox', { name: 'Search workspace' }).fill('Photo of my house')
  await until(
    async () => (await page.locator('.search-message').count()) === 1,
    'offline DM photo search',
  )
  await page.locator('.search-message').click()
  const image = page.locator('.message-attachment img').last()
  await until(
    () => image.evaluate((img) => img.complete && img.naturalWidth === 96),
    'offline image decode',
  )
  assert((await image.getAttribute('src')).startsWith('blob:'))
  await page.getByRole('button', { name: 'Open image', exact: true }).last().click()
  const viewer = page.locator('.media-viewer-media')
  await until(
    () => viewer.evaluate((img) => img.complete && img.naturalWidth === 96),
    'offline full-size image',
  )
  await mkdir('../.data/offline-review', { recursive: true })
  await page.screenshot({ path: '../.data/offline-review/offline-photo-desktop.png' })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.screenshot({ path: '../.data/offline-review/offline-photo-mobile.png' })
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: 'Open conversations', exact: true }).click()
  await page.locator('.sidebar-connection').getByText('Offline', { exact: true }).waitFor()
  await until(async () => {
    const box = await page.locator('.sidebar-connection').boundingBox()
    return box && box.x >= 0 && box.x + box.width <= 390
  }, 'mobile drawer transition')
  await page.screenshot({ path: '../.data/offline-review/sidebar-mobile.png' })
  const footer = await page.locator('.sidebar-connection').boundingBox()
  assert(
    footer && footer.y + footer.height <= 844,
    'Mobile connection indicator must remain visible',
  )
  await page.setViewportSize({ width: 1440, height: 900 })

  // Missing bytes must show a clear state rather than opening an empty viewer.
  await page.evaluate(async (owner) => {
    const db = await new Promise((resolve) => {
      const q = indexedDB.open(`ircord-offline-${owner}`)
      q.onsuccess = () => resolve(q.result)
    })
    await new Promise((resolve, reject) => {
      const tx = db.transaction('files', 'readwrite')
      const files = tx.objectStore('files')
      const q = files.getAll()
      q.onsuccess = () => {
        for (const file of q.result) {
          delete file.blob
          files.put(file)
        }
      }
      tx.oncomplete = resolve
      tx.onabort = () => reject(tx.error)
    })
    db.close()
  }, owner)
  await page.reload()
  await page
    .getByText('This media was not saved on this device.', { exact: true })
    .first()
    .waitFor()
  console.log(
    'Offline reload, Unicode search, older results, DM navigation, and full-size image viewing work with the network disabled.',
  )

  // Keep the archive when the page reloads, then replay a real account update.
  await context.setOffline(false)
  const changed = await fetch(`${control}?run=archive`, { method: 'POST' })
  assert.equal(changed.status, 200)
  const { cursor } = await changed.json()
  await page.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .reconnect(),
  )
  await until(async () => (await archive(page))?.meta?.state.cursor >= cursor, 'archive replay')
  await page.reload()
  await until(
    async () => (await archive(page))?.messages.some((row) => row.id === 'old-photo' && row.pinned),
    'archive survives reload',
  )
  await page.locator('.sidebar-connection').waitFor()
  await until(
    () =>
      page
        .locator('.message-attachment img')
        .last()
        .evaluate((img) => img.complete && img.naturalWidth === 96),
    'saved photo preview after online reload',
  )
  await page.screenshot({ path: '../.data/offline-review/sidebar-desktop.png' })
  await page.locator('.sidebar-connection').click()
  await page.locator('.server-overview').waitFor()
  await page.screenshot({ path: '../.data/offline-review/server-from-indicator.png' })

  // Revoke history while this browser is offline. An expired replay forces a
  // fresh export that removes old local copies and validates retained pins.
  await context.setOffline(true)
  await page.evaluate(() => {
    for (const socket of window.__offlineSockets) socket.close()
  })
  assert.equal(
    (await fetch(`${control}?run=deleted&deletePhoto=true&expire=true`, { method: 'POST' })).status,
    200,
  )
  await context.setOffline(false)
  await page.evaluate(() =>
    document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('chat')
      .reconnect(),
  )
  await until(async () => {
    const data = await archive(page)
    return data?.meta?.complete && data.messages.find((row) => row.id === 'dm-photo')?.deleted
  }, 'snapshot replaces deleted photo')
  stored = await archive(page)
  assert(stored.messages.find((row) => row.id === 'old-photo')?.pinned, 'older pin revalidated')
  assert(
    !stored.files.some((file) => file.refs.some((ref) => ref.id === 'dm-photo')),
    'deleted source must lose its file reference',
  )

  // Deleting the DM removes its pinned media too, then sign-out removes the DB.
  assert.equal(
    (await fetch(`${control}?run=removed&removeDM=true`, { method: 'POST' })).status,
    200,
  )
  await until(async () => {
    const data = await archive(page)
    return (
      data && !data.messages.some((row) => row.conversationId === 'sync-dm') && !data.files.length
    )
  }, 'removed DM purges files and pins')
  await page.evaluate(() => {
    void document
      .querySelector('#__nuxt')
      .__vue_app__.config.globalProperties.$pinia._s.get('auth')
      .logout()
  })
  await page.waitForFunction(
    () =>
      document.querySelector('#__nuxt')?.__vue_app__?.config.globalProperties.$pinia?._s.get('auth')
        ?.status === 'signed-out',
  )
  await until(
    () =>
      page.evaluate(
        async (owner) =>
          !(await indexedDB.databases()).some((db) => db.name === `ircord-offline-${owner}`),
        owner,
      ),
    'account database cleared on sign-out',
  )
  assert.deepEqual(errors, [])
  await peer.close()
  console.log(
    'Reload, replay, expired-cursor replacement, pinned history, deletion cleanup, and sign-out isolation pass.',
  )
} catch (error) {
  if (log) console.error(log)
  if (page) {
    console.error((await page.locator('body').innerText()).slice(-3500))
    await mkdir('../.data/offline-review', { recursive: true })
    await page.screenshot({ path: '../.data/offline-review/failure.png' })
  }
  throw error
} finally {
  await browser?.close()
  frontend.kill('SIGTERM')
}
