type Task = { start: () => void; signal: AbortSignal }
const abortError = () => new DOMException('Link request cancelled.', 'AbortError')

/** Queued work belongs to an active preview. Cancelled waiters never occupy a fetch slot. */
export function createPreviewQueue(limit: number) {
  const pending: Task[] = []
  let active = 0
  function pump() {
    while (active < limit && pending.length) pending.shift()!.start()
  }
  return async function resolveVisible<T>(signal: AbortSignal, run: () => Promise<T>): Promise<T> {
    await new Promise<void>((resolve, reject) => {
      if (signal.aborted) return reject(signal.reason ?? abortError())
      const task: Task = {
        signal,
        start: () => {
          signal.removeEventListener('abort', cancel)
          if (signal.aborted) {
            reject(signal.reason ?? abortError())
            return
          }
          active++
          resolve()
        },
      }
      function cancel() {
        const index = pending.indexOf(task)
        if (index !== -1) pending.splice(index, 1)
        reject(signal.reason ?? abortError())
      }
      signal.addEventListener('abort', cancel, { once: true })
      pending.push(task)
      pump()
    })
    try {
      if (signal.aborted) throw signal.reason ?? abortError()
      return await run()
    } finally {
      active--
      pump()
    }
  }
}

const readers = new Map<string, Set<symbol>>()
/** Shared query consumers can leave independently; only the final reader cancels its work. */
export function retainPreview(key: readonly unknown[], cancel: () => void) {
  const id = JSON.stringify(key)
  const token = Symbol()
  const owners = readers.get(id) ?? new Set<symbol>()
  readers.set(id, owners)
  owners.add(token)
  return () => {
    if (!owners.delete(token) || owners.size) return
    readers.delete(id)
    cancel()
  }
}
