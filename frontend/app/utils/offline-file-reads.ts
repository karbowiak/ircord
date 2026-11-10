/** Share pending reads, not Blob results. Only a few lookups enter the serial archive worker. */
export function createOfflineFileReads<T>(
  read: (url: string) => Promise<T>,
  { concurrency = 4, capacity = 1024 } = {},
) {
  type Reader = { resolve: (value: T) => void; reject: (error: unknown) => void }
  type Entry = { url: string; readers: Set<Reader>; started: boolean; generation: number }
  const entries = new Map<string, Entry>()
  const queued = new Set<Entry>()
  let generation = 0
  let active = 0
  let scheduled = false
  const cancelled = () => new DOMException('Offline file read cancelled.', 'AbortError')

  function forget(entry: Entry) {
    if (entries.get(entry.url) === entry) entries.delete(entry.url)
    queued.delete(entry)
  }
  function schedule() {
    if (scheduled) return
    scheduled = true
    queueMicrotask(() => {
      scheduled = false
      for (const entry of queued) {
        if (active >= concurrency) break
        queued.delete(entry)
        if (!entry.readers.size) {
          forget(entry)
          continue
        }
        entry.started = true
        active++
        void Promise.resolve()
          .then(() => {
            if (entry.generation !== generation || !entry.readers.size) throw cancelled()
            return read(entry.url)
          })
          .then(
            (value) => {
              const readers = [...entry.readers]
              forget(entry)
              entry.readers.clear()
              for (const reader of readers) reader.resolve(value)
            },
            (error: unknown) => {
              const readers = [...entry.readers]
              forget(entry)
              entry.readers.clear()
              for (const reader of readers) reader.reject(error)
            },
          )
          .finally(() => {
            forget(entry)
            entry.readers.clear()
            if (entry.generation === generation) {
              active--
              schedule()
            }
          })
      }
    })
  }
  function invalidate(url?: string) {
    const affected = url === undefined ? [...entries.values()] : [entries.get(url)]
    for (const entry of affected) {
      if (!entry) continue
      forget(entry)
      for (const reader of entry.readers) reader.reject(cancelled())
      entry.readers.clear()
    }
  }
  return {
    read(url: string, signal?: AbortSignal): Promise<T> {
      if (signal?.aborted) return Promise.reject(cancelled())
      let entry = entries.get(url)
      if (!entry) {
        if (entries.size >= capacity)
          return Promise.reject(new Error('Offline file reads are busy.'))
        entry = { url, readers: new Set(), started: false, generation }
        entries.set(url, entry)
        queued.add(entry)
      }
      const shared = entry
      return new Promise<T>((resolve, reject) => {
        const reader: Reader = {
          resolve(value) {
            signal?.removeEventListener('abort', abort)
            resolve(value)
          },
          reject(error) {
            signal?.removeEventListener('abort', abort)
            reject(error)
          },
        }
        function abort() {
          shared.readers.delete(reader)
          reader.reject(cancelled())
          if (!shared.started && !shared.readers.size) forget(shared)
        }
        shared.readers.add(reader)
        signal?.addEventListener('abort', abort, { once: true })
        schedule()
      })
    },
    invalidate,
    reset() {
      invalidate()
      generation++
      active = 0
    },
  }
}
