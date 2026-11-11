export type RecoveryStatus = 'connecting' | 'syncing' | 'connected' | 'reconnecting' | 'offline'

export const RECOVERY = {
  ready: 10_000,
  replayIdle: 12_000,
  replayTotal: 45_000,
  snapshot: 15_000,
  session: 5000,
  history: 15_000,
  grace: 18_000,
  staleAfter: 5000,
  coalesce: 500,
} as const

/** Abort the operation and settle its caller even if an adapter ignores cancellation. */
export function boundedRead<T>(
  read: (signal: AbortSignal) => Promise<T>,
  controller: AbortController,
  milliseconds: number,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const signal = controller.signal
    const cleanup = () => {
      clearTimeout(timer)
      signal.removeEventListener('abort', abort)
    }
    const abort = () => {
      cleanup()
      reject(signal.reason)
    }
    const timer = setTimeout(
      () => controller.abort(new DOMException('Recovery request timed out', 'TimeoutError')),
      milliseconds,
    )
    signal.addEventListener('abort', abort, { once: true })
    if (signal.aborted) abort()
    else
      Promise.resolve()
        .then(() => {
          signal.throwIfAborted()
          return read(signal)
        })
        .then(
          (value) => {
            cleanup()
            resolve(value)
          },
          (error) => {
            cleanup()
            reject(error)
          },
        )
    // The caller receives data only; it must check its attempt before installing it.
  })
}

/** One socket generation, one backoff timer, and bounded handshake/snapshot phases. */
export function createRealtimeRecovery<Snapshot>(options: {
  enabled: () => boolean
  online: () => boolean
  visible: () => boolean
  open: () => WebSocket
  socket: (socket: WebSocket | undefined) => void
  receive: (raw: string) => void
  snapshot: (signal: AbortSignal) => Promise<Snapshot>
  invalidate: () => void
  install: (snapshot: Snapshot) => void
  session: (signal: AbortSignal) => Promise<boolean>
  expired: () => void
  status: (status: RecoveryStatus) => void
  failure: (failed: boolean) => void
  lost: () => void
  synced: (resumed: boolean) => void
}) {
  let current: WebSocket | undefined
  let generation = 0
  let stopped = false
  let started = false
  let state: RecoveryStatus = 'connecting'
  let attempts = 0
  let synchronizedOnce = false
  let needsSnapshot = false
  let replacedSnapshot = false
  let request: AbortController | undefined
  let session: AbortController | undefined
  let retry: ReturnType<typeof setTimeout> | undefined
  let deadline: ReturnType<typeof setTimeout> | undefined
  let total: ReturnType<typeof setTimeout> | undefined
  let grace: ReturnType<typeof setTimeout> | undefined
  let elapsed = 0
  let runningSince: number | undefined
  let hiddenAt: number | undefined
  let frozen = false
  let lastResume = -Infinity
  let lastAttempt = -Infinity

  const allowed = () => started && !stopped && options.enabled()
  const suspended = () => frozen || hiddenAt !== undefined || !options.visible()
  function updateGrace() {
    clearTimeout(grace)
    if (runningSince !== undefined) elapsed += Date.now() - runningSince
    runningSince = undefined
    if (state === 'connected' || !allowed()) {
      elapsed = 0
      options.failure(false)
      return
    }
    options.failure(elapsed >= RECOVERY.grace)
    if (options.online() && !suspended()) {
      runningSince = Date.now()
      if (elapsed < RECOVERY.grace) grace = setTimeout(updateGrace, RECOVERY.grace - elapsed)
    }
  }
  function setStatus(value: RecoveryStatus) {
    state = value
    options.status(value)
    updateGrace()
  }
  function release() {
    generation++
    clearTimeout(retry)
    clearTimeout(deadline)
    clearTimeout(total)
    request?.abort()
    session?.abort()
    request = session = undefined
    if (current) {
      current.onmessage = current.onerror = current.onclose = null
      current.close()
      current = undefined
      options.socket(undefined)
      options.lost()
    }
  }
  function checkSession(ticket: number) {
    const controller = (session = new AbortController())
    void boundedRead(options.session, controller, RECOVERY.session)
      .then((valid) => {
        if (!valid && ticket === generation && allowed() && !controller.signal.aborted) {
          dispose()
          options.expired()
        }
      })
      .catch(() => {})
      .finally(() => {
        if (session === controller) session = undefined
      })
  }
  function failed(verifySession = true) {
    if (!allowed()) return
    release()
    options.lost()
    setStatus(options.online() ? 'reconnecting' : 'offline')
    if (!options.online() || frozen) return
    const delay = Math.min(15_000, 800 * 2 ** Math.min(attempts++, 5)) + Math.random() * 250
    const ticket = generation
    retry = setTimeout(() => {
      if (ticket === generation) attempt()
    }, delay)
    // Authentication cannot block scheduling; obsolete responses cannot expire a new socket.
    if (verifySession) checkSession(generation)
  }
  function arm(milliseconds: number) {
    clearTimeout(deadline)
    const ticket = generation
    deadline = setTimeout(() => {
      if (ticket === generation) failed()
    }, milliseconds)
  }
  function open() {
    if (!allowed() || frozen) return
    release()
    lastAttempt = Date.now()
    const ticket = generation
    setStatus(synchronizedOnce || attempts ? 'reconnecting' : 'connecting')
    try {
      current = options.open()
      options.socket(current)
      current.onmessage = (event) => {
        if (ticket !== generation || !allowed()) return
        try {
          options.receive(event.data)
        } catch {
          resync()
        }
      }
      current.onerror = () => {
        if (ticket === generation) failed()
      }
      current.onclose = (event) => {
        if (ticket !== generation || !allowed()) return
        if (event.code === 1008) {
          needsSnapshot = true
          options.invalidate()
        }
        failed()
      }
      arm(RECOVERY.ready)
    } catch {
      failed()
    }
  }
  function attempt() {
    if (!allowed()) return
    release()
    if (!options.online()) return setStatus('offline')
    if (frozen) return
    lastAttempt = Date.now()
    if (!needsSnapshot) return open()
    setStatus('syncing')
    const ticket = generation
    const controller = (request = new AbortController())
    void boundedRead(options.snapshot, controller, RECOVERY.snapshot)
      .then((snapshot) => {
        if (ticket !== generation || !allowed() || controller.signal.aborted) return
        options.install(snapshot)
        needsSnapshot = false
        replacedSnapshot = true
        open()
      })
      .catch(() => {
        if (ticket === generation && allowed()) failed()
      })
  }
  function ready(sync = true) {
    if (!allowed() || !current || state === 'connected' || state === 'syncing') return
    if (!sync) return synced()
    setStatus('syncing')
    arm(RECOVERY.replayIdle)
    clearTimeout(total)
    const ticket = generation
    total = setTimeout(() => {
      if (ticket === generation) failed()
    }, RECOVERY.replayTotal)
  }
  function progress() {
    if (state === 'syncing' && current) arm(RECOVERY.replayIdle)
  }
  function synced() {
    if (!allowed() || !current || state === 'connected') return
    clearTimeout(deadline)
    clearTimeout(total)
    attempts = 0
    const resumed = synchronizedOnce || replacedSnapshot
    synchronizedOnce = true
    replacedSnapshot = false
    setStatus('connected')
    options.synced(resumed)
  }
  function resync() {
    if (!allowed()) return
    needsSnapshot = true
    options.invalidate()
    attempt()
  }
  function reconnect() {
    if (allowed()) attempt()
  }
  function resume(force = false) {
    if (!allowed() || suspended() || !options.online()) return
    const now = Date.now()
    // Offline released the attempt and its timer, so its only online trigger must never be dropped.
    if (
      state !== 'offline' &&
      (now - lastResume < RECOVERY.coalesce || now - lastAttempt < RECOVERY.coalesce)
    )
      return
    if (state === 'connected' && !force) return
    lastResume = now
    attempt()
  }
  function visibility(hidden: boolean) {
    if (hidden) {
      hiddenAt ??= Date.now()
    } else {
      const stale = hiddenAt !== undefined && Date.now() - hiddenAt >= RECOVERY.staleAfter
      if (hiddenAt !== undefined) {
        // Coalesce events within this foreground episode, never with a background attempt.
        attempts = 0
        lastResume = lastAttempt = -Infinity
      }
      hiddenAt = undefined
      resume(stale)
    }
    updateGrace()
  }
  function freeze() {
    if (!allowed()) return
    frozen = true
    release()
    setStatus(options.online() ? 'reconnecting' : 'offline')
  }
  function thaw() {
    if (!frozen) return
    frozen = false
    attempts = 0
    lastResume = lastAttempt = -Infinity
    resume(true)
    updateGrace()
  }
  function offline() {
    if (!allowed()) return
    release()
    options.lost()
    setStatus('offline')
  }
  function dispose() {
    if (stopped) return
    stopped = true
    release()
    clearTimeout(grace)
    options.failure(false)
  }
  return {
    start() {
      if (!started && !stopped) {
        started = true
        attempt()
      }
    },
    reconnect,
    resync,
    ready,
    progress,
    synced,
    resume,
    visibility,
    freeze,
    thaw,
    offline,
    dispose,
  }
}
