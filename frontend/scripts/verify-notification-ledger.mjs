import { createServer } from 'node:http'
import { readFile } from 'node:fs/promises'
import { Transpiler } from 'bun'
import { chromium } from 'playwright'

const source = await readFile(
  new URL('../app/utils/notification-ledger.ts', import.meta.url),
  'utf8',
)
const ledger = new Transpiler({ loader: 'ts', target: 'browser' }).transformSync(source)
const worker = await readFile(new URL('../public/sw.js', import.meta.url), 'utf8')
const server = createServer((request, response) => {
  if (request.url === '/ledger.js') {
    response.setHeader('Content-Type', 'text/javascript')
    response.end(ledger)
  } else if (request.url === '/sw.js') {
    response.setHeader('Content-Type', 'text/javascript')
    response.end(
      `${worker}\nself.addEventListener('message', event => { event.waitUntil((async () => {
        if (event.data?.abort) {
          const original = IDBDatabase.prototype.transaction
          IDBDatabase.prototype.transaction = function (...args) { const tx = original.apply(this, args); queueMicrotask(() => tx.abort()); return tx }
          try { return await Promise.all([claimAlert(event.data.key), claimAlert(event.data.key)]) }
          finally { IDBDatabase.prototype.transaction = original }
        }
        return claimAlert(event.data)
      })().then(value => event.ports[0].postMessage(value))) })`,
    )
  } else {
    response.setHeader('Content-Type', 'text/html')
    response.end('<!doctype html><title>Notification ledger test</title>')
  }
})
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
const browser = await chromium.launch({ headless: true })
try {
  const context = await browser.newContext()
  const first = await context.newPage()
  const second = await context.newPage()
  const origin = `http://127.0.0.1:${server.address().port}`
  await Promise.all([first.goto(origin), second.goto(origin)])
  // Version 1 has primitive timestamps and no index. Migration must retain its keys without
  // rebuilding a large index in the upgrade transaction.
  await first.evaluate(async () => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open('ircord-notification-ledger', 1)
      request.onupgradeneeded = () => request.result.createObjectStore('alerts')
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    await new Promise((resolve, reject) => {
      const tx = db.transaction('alerts', 'readwrite')
      const store = tx.objectStore('alerts')
      for (let i = 0; i < 100; i++) store.add(Date.now() - 8 * 86_400_000, `old-${String(i).padStart(3, '0')}`)
      store.add(Date.now(), 'zz-retained')
      tx.oncomplete = resolve
      tx.onabort = () => reject(tx.error)
    })
    db.close()
  })
  await first.evaluate(async () => {
    await navigator.serviceWorker.register('/sw.js')
    await navigator.serviceWorker.ready
  })
  const pageClaim = (page, key) =>
    page.evaluate(async (key) => (await import('/ledger.js')).claimNotification(key), key)
  const workerClaim = (key) =>
    first.evaluate(async (key) => {
      const registration = await navigator.serviceWorker.ready
      return await new Promise((resolve) => {
        const channel = new MessageChannel()
        channel.port1.onmessage = (event) => resolve(event.data)
        registration.active.postMessage(key, [channel.port2])
      })
    }, key)
  if (!(await pageClaim(first, 'first-new-key'))) throw new Error('Initial migration claim failed')
  const legacyCount = await first.evaluate(async () => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open('ircord-notification-ledger', 2)
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    const count = await new Promise((resolve) => {
      const request = db.transaction('alerts').objectStore('alerts').count()
      request.onsuccess = () => resolve(request.result)
    })
    db.close()
    return count
  })
  if (legacyCount !== 69) throw new Error(`Migration did not stop after 32 records: ${legacyCount}`)
  if (await workerClaim('zz-retained')) throw new Error('Legacy dedupe key was lost during migration')
  const claims = await Promise.all([
    pageClaim(first, 'generation:message:os'),
    pageClaim(second, 'generation:message:os'),
    workerClaim('generation:message:os'),
  ])
  if (claims.filter(Boolean).length !== 1) throw new Error(`Expected one OS claim, got ${claims}`)
  if (!(await workerClaim('generation:message:sound')))
    throw new Error('Sound-only claim was suppressed by OS delivery')
  if (await pageClaim(first, 'generation:message:sound'))
    throw new Error('Duplicate sound claim succeeded')
  if (!(await pageClaim(second, 'next-generation:message:os')))
    throw new Error('New generation was suppressed')
  if (await pageClaim(second, 'zz-retained'))
    throw new Error('A migrated legacy key no longer deduplicates across contexts')
  const migrated = await first.evaluate(async () => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open('ircord-notification-ledger', 2)
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    const value = await new Promise((resolve) => {
      const request = db.transaction('alerts-by-time').objectStore('alerts-by-time').get('zz-retained')
      request.onsuccess = () => resolve(request.result)
    })
    db.close()
    return value
  })
  if (typeof migrated !== 'number') throw new Error('The retained legacy key was not migrated')
  await first.evaluate(async () => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open('ircord-notification-ledger', 2)
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    await new Promise((resolve, reject) => {
      const tx = db.transaction('alerts-by-time', 'readwrite')
      const store = tx.objectStore('alerts-by-time')
      for (let i = 0; i < 100; i++) store.add(Date.now() - 8 * 86_400_000, `expired-${i}`)
      tx.oncomplete = resolve
      tx.onabort = () => reject(tx.error)
    })
    db.close()
  })
  await workerClaim('cleanup-trigger')
  const remainingExpired = await first.evaluate(async () => {
    const db = await new Promise((resolve, reject) => {
      const request = indexedDB.open('ircord-notification-ledger', 2)
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    const count = await new Promise((resolve) => {
      const request = db.transaction('alerts-by-time').objectStore('alerts-by-time')
        .index('timestamp').count(IDBKeyRange.upperBound(Date.now() - 7 * 86_400_000, true))
      request.onsuccess = () => resolve(request.result)
    })
    db.close()
    return count
  })
  if (remainingExpired !== 68) throw new Error(`Expired cleanup was not bounded: ${remainingExpired}`)
  const fallback = await first.evaluate(async () => {
    const original = IDBDatabase.prototype.transaction
    IDBDatabase.prototype.transaction = function (...args) {
      const tx = original.apply(this, args)
      queueMicrotask(() => tx.abort())
      return tx
    }
    try {
      const { claimNotification } = await import('/ledger.js')
      return await Promise.all([claimNotification('abort-key'), claimNotification('abort-key')])
    } finally {
      IDBDatabase.prototype.transaction = original
    }
  })
  if (fallback.filter(Boolean).length !== 1) throw new Error(`Abort fallback failed: ${fallback}`)
  const workerFallback = await workerClaim({ abort: true, key: 'worker-abort-key' })
  if (workerFallback.filter(Boolean).length !== 1)
    throw new Error(`Worker abort fallback failed: ${workerFallback}`)
  await context.close()
  console.log('Notification ledger: atomic claims, bounded migration and cleanup, and abort fallback passed')
} finally {
  await browser.close()
  await new Promise((resolve) => server.close(resolve))
}
