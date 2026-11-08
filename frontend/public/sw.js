// IRCord service worker:
// - Media cache: GIFs, emotes, avatars and the emoji dataset are immutable at their URLs, so
//   they are served cache-first and never fetched twice.
// - App shell: the page is network-first with a cached fallback, and hashed build assets are
//   cached once, so the installed app opens offline. The API and WebSocket pass through.
// - Push notifications (below).
const CACHE = 'ircord-media-v2'
const SHELL = 'ircord-shell-v1'
const MAX_ENTRIES = 1500
// The Klipy hosts are kept in step with klipyMediaHosts in shared/utils/gif-hosts.ts.
const MEDIA_HOSTS = new Set(['static.klipy.com', 'media.klipy.com', 'cdn.klipy.com', 'cdn.7tv.app'])
const LOCAL_MEDIA = /^\/(avatars|data|emotes)\//

self.addEventListener('install', () => self.skipWaiting())
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter(
              (k) =>
                (k.startsWith('ircord-media-') && k !== CACHE) ||
                (k.startsWith('ircord-shell-') && k !== SHELL),
            )
            .map((k) => caches.delete(k)),
        ),
      )
      .then(() => self.clients.claim()),
  )
})

function cacheable(request) {
  if (request.method !== 'GET') return false
  const url = new URL(request.url)
  if (url.origin === self.location.origin) return LOCAL_MEDIA.test(url.pathname)
  return url.protocol === 'https:' && MEDIA_HOSTS.has(url.hostname)
}

let writes = 0
async function trim(cache) {
  // Cache keys keep insertion order, so dropping from the front evicts the oldest entries.
  const keys = await cache.keys()
  for (const key of keys.slice(0, Math.max(0, keys.length - MAX_ENTRIES))) await cache.delete(key)
}

async function respond(request) {
  const cache = await caches.open(CACHE)
  const hit = await cache.match(request, { ignoreVary: true })
  if (hit) return hit
  const url = new URL(request.url)
  // Fetch cross-origin media in CORS mode (both CDNs allow it) so the cached copy is readable
  // and does not count as a padded opaque response against the storage quota.
  const response = await fetch(
    url.origin === self.location.origin
      ? request
      : new Request(url, { mode: 'cors', credentials: 'omit', referrerPolicy: 'no-referrer' }),
  ).catch(() => fetch(request))
  if (response.ok && response.type !== 'opaque') {
    await cache.put(request, response.clone()).catch(() => {})
    if (++writes % 50 === 0) void trim(cache)
  }
  return response
}

// ---- App shell -----------------------------------------------------------------------------------

/** The page: try the network briefly, fall back to the last good copy when offline. */
async function page(request) {
  const cache = await caches.open(SHELL)
  try {
    const response = await Promise.race([
      fetch(request),
      new Promise((_, reject) => setTimeout(() => reject(new Error('timeout')), 4000)),
    ])
    if (response.ok && (response.headers.get('content-type') ?? '').includes('text/html'))
      await cache.put('/', response.clone()).catch(() => {})
    return response
  } catch {
    return (await cache.match('/')) ?? Response.error()
  }
}
/** Build assets: only immutable (hashed, production) files are kept; dev modules never are. */
async function asset(request) {
  const cache = await caches.open(SHELL)
  const hit = await cache.match(request)
  if (hit) return hit
  const response = await fetch(request)
  if (response.ok && /immutable/.test(response.headers.get('cache-control') ?? ''))
    await cache.put(request, response.clone()).catch(() => {})
  return response
}
const SHELL_FILES = /^\/(favicon\.svg|manifest\.webmanifest|icons\/)/

// The favicon, manifest and app icons keep their URLs when the artwork changes, so they are served
// from the cache for an instant, offline-capable start and refreshed in the background: a release
// that changes the brand mark shows it from the next load instead of leaving the old one stuck.
// The refresh revalidates past the browser's HTTP cache (a cheap 304 when nothing changed), and
// never stores the HTML page the server answers for a path that no longer exists.
async function shellFile(event, request) {
  const cache = await caches.open(SHELL)
  const refresh = fetch(request, { cache: 'no-cache' }).then((response) => {
    const html = (response.headers.get('content-type') ?? '').startsWith('text/html')
    if (response.ok && !html) event.waitUntil(cache.put(request, response.clone()).catch(() => {}))
    return response
  })
  const hit = await cache.match(request)
  if (hit) {
    event.waitUntil(refresh.catch(() => {}))
    return hit
  }
  return refresh
}

self.addEventListener('fetch', (event) => {
  const request = event.request
  if (request.method !== 'GET') return
  const url = new URL(request.url)
  if (request.mode === 'navigate' && url.origin === self.location.origin)
    event.respondWith(page(request))
  else if (url.origin === self.location.origin && url.pathname.startsWith('/_nuxt/builds/'))
    return // The build manifest says which version is live; never answer it from the cache.
  else if (url.origin === self.location.origin && url.pathname.startsWith('/_nuxt/'))
    event.respondWith(asset(request))
  else if (url.origin === self.location.origin && SHELL_FILES.test(url.pathname))
    event.respondWith(shellFile(event, request))
  else if (cacheable(request)) event.respondWith(respond(request))
})

// ---- Push notifications ------------------------------------------------------------------------
// The backend pushes highlights and direct messages: { title, body, conversationId, tag, badge }.

async function focusedClient() {
  const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
  return windows.find((client) => client.focused && client.visibilityState === 'visible')
}

self.addEventListener('push', (event) => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = { title: 'IRCord', body: event.data ? event.data.text() : '' }
  }
  event.waitUntil(
    (async () => {
      if (typeof data.badge === 'number' && self.navigator.setAppBadge)
        await (
          data.badge ? self.navigator.setAppBadge(data.badge) : self.navigator.clearAppBadge()
        ).catch(() => {})
      // The open, focused app already shows new messages; don't interrupt it.
      if (data.tag !== 'test' && (await focusedClient())) return
      await self.registration.showNotification(data.title || 'IRCord', {
        body: data.body || '',
        tag: data.tag,
        renotify: !!data.tag,
        icon: '/icons/icon-192.png',
        badge: '/icons/badge-96.png',
        data: { conversationId: data.conversationId },
      })
    })(),
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const conversationId = event.notification.data?.conversationId
  event.waitUntil(
    (async () => {
      const windows = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
      const client = windows.find((w) => new URL(w.url).origin === self.location.origin)
      if (client) {
        await client.focus()
        if (conversationId) client.postMessage({ type: 'open-conversation', conversationId })
        return
      }
      await self.clients.openWindow(
        conversationId ? `/?c=${encodeURIComponent(conversationId)}` : '/',
      )
    })(),
  )
})

// A browser can rotate the subscription; tell any open window so it re-registers.
self.addEventListener('pushsubscriptionchange', (event) => {
  event.waitUntil(
    self.clients
      .matchAll({ type: 'window' })
      .then((windows) =>
        windows.forEach((w) => w.postMessage({ type: 'push-subscription-changed' })),
      ),
  )
})
