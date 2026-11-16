/** How often a busy workspace writes its checkpoint at most. */
export const CHECKPOINT_INTERVAL = 60_000
/** How long the first change waits, so a burst of events lands in one write. */
export const CHECKPOINT_SETTLE = 1_000

interface Timers {
  now(): number
  setTimeout(run: () => void, ms: number): unknown
  clearTimeout(handle: unknown): void
}
const browserTimers: Timers = {
  now: () => Date.now(),
  setTimeout: (run, ms) => setTimeout(run, ms),
  clearTimeout: (handle) => clearTimeout(handle as ReturnType<typeof setTimeout>),
}

/**
 * Coalesces workspace checkpoint writes. Each write serialises the whole workspace, so changes
 * save shortly after the first one and then at most once per interval; hiding or leaving the
 * page saves at once. `save` returns undefined when the workspace cannot be saved yet; that
 * attempt does not use up the interval, so the next change saves soon after.
 */
export function createCheckpointSchedule(
  save: () => boolean | undefined,
  timers: Timers = browserTimers,
) {
  let timer: unknown
  let last = -Infinity
  function cancel() {
    if (timer !== undefined) timers.clearTimeout(timer)
    timer = undefined
  }
  /** Saves now, replacing any save still waiting. */
  function flush() {
    cancel()
    if (save() !== undefined) last = timers.now()
  }
  /** Notes a change; it saves after the settle delay, or once the interval has passed. */
  function changed() {
    if (timer !== undefined) return
    const wait = Math.max(CHECKPOINT_SETTLE, last + CHECKPOINT_INTERVAL - timers.now())
    timer = timers.setTimeout(flush, wait)
  }
  return { changed, flush, cancel }
}
