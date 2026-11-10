const DB = 'ircord-notification-ledger'
const LEGACY = 'alerts'
const STORE = 'alerts-by-time'
const META = 'migration'
const RETAIN_MS = 7 * 24 * 60 * 60 * 1000
const CLEANUP_LIMIT = 32
const fallback = new Map<string, number>()

function remember(key: string, now: number) {
  fallback.set(key, now)
  if (fallback.size > 1000) fallback.delete(fallback.keys().next().value!)
}

/** Atomically claim one presentation across tabs and the service worker. */
export async function claimNotification(key: string): Promise<boolean> {
  const now = Date.now()
  const seen = fallback.get(key)
  if (seen !== undefined && seen >= now - RETAIN_MS) return false
  try {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open(DB, 2)
      request.onupgradeneeded = () => {
        const db = request.result
        if (!db.objectStoreNames.contains(LEGACY)) db.createObjectStore(LEGACY)
        db.createObjectStore(STORE).createIndex('timestamp', '')
        db.createObjectStore(META)
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
    const claimed = await new Promise<boolean>((resolve, reject) => {
      const transaction = db.transaction([LEGACY, STORE, META], 'readwrite')
      const legacy = transaction.objectStore(LEGACY)
      const store = transaction.objectStore(STORE)
      const meta = transaction.objectStore(META)
      let claimed = false
      // Old keys remain authoritative while the legacy store migrates in bounded batches.
      const old = legacy.getKey(key)
      old.onsuccess = () => {
        if (old.result !== undefined) return
        const add = store.add(now, key)
        add.onsuccess = () => (claimed = true)
        add.onerror = (event) => {
          if (add.error?.name === 'ConstraintError') {
            event.preventDefault()
            event.stopPropagation()
          }
        }
      }
      const expired = store.index('timestamp').openCursor(IDBKeyRange.upperBound(now - RETAIN_MS, true))
      let removed = 0
      expired.onsuccess = () => {
        const entry = expired.result
        if (!entry) return
        entry.delete()
        if (++removed < CLEANUP_LIMIT) entry.continue()
      }
      const progress = meta.get('legacy-cursor')
      progress.onsuccess = () => {
        const range = progress.result === undefined ? undefined : IDBKeyRange.lowerBound(progress.result, true)
        const cursor = legacy.openCursor(range)
        let moved = 0
        cursor.onsuccess = () => {
          const entry = cursor.result
          if (!entry) return void meta.delete('legacy-cursor')
          if (typeof entry.value === 'number' && entry.value >= now - RETAIN_MS) {
            const add = store.add(entry.value, entry.primaryKey)
            add.onerror = (event) => {
              if (add.error?.name === 'ConstraintError') {
                event.preventDefault()
                event.stopPropagation()
              }
            }
          }
          entry.delete()
          if (++moved < CLEANUP_LIMIT) entry.continue()
          else meta.put(entry.primaryKey, 'legacy-cursor')
        }
      }
      transaction.oncomplete = () => resolve(claimed)
      transaction.onabort = () => reject(transaction.error)
    }).finally(() => db.close())
    remember(key, now)
    return claimed
  } catch {
    // Storage failures cannot coordinate with other tabs or the worker, but must not drop alerts.
    const last = fallback.get(key)
    if (last !== undefined && last >= now - RETAIN_MS) return false
    remember(key, now)
    return true
  }
}
