export interface LatestQueueOptions<T, R> {
  /** Quiet period after the last push before saving starts. */
  delay: number
  /** Called when a value is saved and nothing newer is waiting. */
  onSaved: (value: T, result: R) => void
  /** Called when the newest value failed to save. */
  onError: (error: unknown) => void
}

/**
 * Debounced, single-flight, latest-wins saving for settings that change rapidly.
 * Pushes during the quiet period coalesce, only one request runs at a time, and
 * after it finishes only the newest pushed value is sent; intermediate values are skipped.
 */
export function createLatestQueue<T, R>(
  send: (value: T) => Promise<R>,
  options: LatestQueueOptions<T, R>,
) {
  let latest: { value: T } | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let running = false

  async function run() {
    timer = undefined
    if (running || !latest) return
    running = true
    const { value } = latest
    latest = undefined
    try {
      const result = await send(value)
      if (!latest) options.onSaved(value, result)
    } catch (error) {
      // A newer value is already queued; its own save reports the outcome.
      if (!latest) options.onError(error)
    } finally {
      running = false
      if (latest && !timer) void run()
    }
  }

  return {
    push(value: T) {
      latest = { value }
      clearTimeout(timer)
      timer = setTimeout(() => void run(), options.delay)
    },
    /** True while a value is waiting or being saved. */
    get busy() {
      return running || !!latest
    },
  }
}
