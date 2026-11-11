import { nextTick, shallowReactive, watch, type Ref } from 'vue'
import type { Router } from 'vue-router'

export type ViewState =
  string | number | boolean | null | ViewState[] | { [key: string]: ViewState }
type Snapshot = { values: Record<string, ViewState>; dialogs: number; ancestors: number[] }
type Entry = { close: () => unknown; guard?: () => boolean; modal: boolean }
const stateKey = 'ircordViews'
const equal = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)
const clone = <T extends ViewState>(value: T): T => JSON.parse(JSON.stringify(value))
const popHandlers = new Set<(event: PopStateEvent) => void>()
let listening = false
const dispatchPop = (event: PopStateEvent) => {
  for (const handler of popHandlers) handler(event)
}

/** Install before Vue Router: window-target events run listeners in registration order. */
export function installViewHistoryEvents() {
  if (listening) return
  window.addEventListener('popstate', dispatchPop)
  listening = true
}

export function removeViewHistoryEvents() {
  window.removeEventListener('popstate', dispatchPop)
  listening = false
}

/** Navigation identifiers only. Form contents and component closures never enter history. */
export function createViewHistory(router: Router) {
  const history = router.options.history
  const open = shallowReactive<Entry[]>([])
  const owners = new Map<
    string,
    { read: () => ViewState; write: (value: ViewState) => void; initial: ViewState }
  >()
  let restoring = false
  let scheduled = false
  let disposed = false
  let desired: Snapshot | undefined
  let routePop = false
  let observed = Number(window.history.state?.position ?? 0)
  let previousListening: boolean | undefined
  let resumeTimer: ReturnType<typeof setTimeout> | undefined
  let focusTimer: ReturnType<typeof setTimeout> | undefined
  let pending: { resolve: () => void; promise: Promise<void> } | undefined

  function read(): Snapshot | undefined {
    const value = window.history.state?.[stateKey] as Snapshot | undefined
    return value && typeof value.values === 'object' && Array.isArray(value.ancestors)
      ? value
      : undefined
  }
  function snapshot(ancestors = read()?.ancestors ?? []): Snapshot {
    return {
      values: Object.fromEntries(
        [...owners].flatMap(([key, owner]) => {
          const value = owner.read()
          return equal(value, owner.initial) ? [] : [[key, value]]
        }),
      ),
      dialogs: open.length,
      ancestors: ancestors.slice(0, open.length),
    }
  }
  function replace(value: Snapshot) {
    history.replace(router.currentRoute.value.fullPath, { [stateKey]: value })
    observed = Number(window.history.state.position)
  }
  function pauseRouter() {
    // Let Vue Router update its position/location bookkeeping, without routing the same page.
    previousListening ??= router.listening
    router.listening = false
    clearTimeout(resumeTimer)
    // Browser event listeners have microtask checkpoints between them. Resume only after
    // Vue Router's own popstate listener has updated its history state.
    resumeTimer = setTimeout(() => {
      if (!disposed) router.listening = previousListening ?? true
      previousListening = undefined
    }, 0)
  }
  function travel(position: number) {
    const delta = position - Number(window.history.state.position)
    if (!delta) return Promise.resolve()
    if (pending) return pending.promise
    let resolve!: () => void
    const promise = new Promise<void>((done) => {
      resolve = done
    })
    pending = { resolve, promise }
    history.go(delta, false)
    return promise
  }
  async function apply(value: Snapshot | undefined) {
    desired = value
    restoring = true
    for (const [key, owner] of owners) owner.write(value?.values[key] ?? owner.initial)
    // Newly mounted owners (settings sections, media) hydrate from the same snapshot.
    await nextTick()
    await nextTick()
    restoring = false
    desired = undefined
  }
  async function flush() {
    if (disposed || restoring) return
    if (pending) await pending.promise
    if (disposed || restoring) return
    const previous = read()
    const next = snapshot()
    if (!previous) return replace(next)
    if (equal(previous.values, next.values) && previous.dialogs === next.dialogs) return
    if (next.dialogs < previous.dialogs) {
      const start = previous.ancestors[next.dialogs]
      if (start !== undefined) {
        restoring = true
        await travel(start - 1)
        restoring = false
        // Explicit close consumes the dismissed view's subviews. Forward can restore them.
        const current = snapshot()
        if (current.dialogs > next.dialogs) {
          // A new view opened while the browser was consuming the close. Give it its own
          // entry after the traversal, rather than overwriting the underlying page.
          replace(next)
          schedule()
        } else replace(current)
        return
      }
    }
    const position = Number(window.history.state.position) + 1
    while (next.ancestors.length < next.dialogs) next.ancestors.push(position)
    history.push(router.currentRoute.value.fullPath, { [stateKey]: next })
    observed = Number(window.history.state.position)
  }
  function schedule() {
    if (scheduled || restoring || disposed) return
    scheduled = true
    void nextTick().then(async () => {
      await nextTick()
      scheduled = false
      await flush()
    })
  }
  function remove(entry: Entry) {
    const index = open.indexOf(entry)
    if (index < 0) return
    open.splice(index, 1)
    schedule()
  }
  function closeTo(count: number) {
    while (open.length > count) {
      const entry = open.at(-1)!
      if (entry.close() === false) return false
      remove(entry)
    }
    return true
  }
  function restoreViewFocus() {
    const active = document.activeElement as HTMLElement | null
    const panels = [...document.querySelectorAll<HTMLElement>('[role="dialog"]')].filter(
      (panel) => panel.getClientRects().length && !panel.closest('[inert], [aria-hidden="true"]'),
    )
    const panel = panels.at(-1)
    if (!panel) return
    if (
      active &&
      active !== document.body &&
      active.isConnected &&
      panel.contains(active) &&
      active.getClientRects().length &&
      !active.closest('[inert], [aria-hidden="true"], [data-state="closed"]')
    )
      return
    // WebKit can return focus to the underlying conversation during same-page traversal.
    // Restore it once to the selected view, so Escape and keyboard navigation reach it.
    for (const selector of [
      '[autofocus]',
      'input:not([type="hidden"]):not(:disabled), textarea:not(:disabled)',
      'button:not(:disabled), a[href], [tabindex="0"]',
    ]) {
      const control = [...panel.querySelectorAll<HTMLElement>(selector)].find(
        (element) =>
          element.getClientRects().length && !element.closest('[inert], [aria-hidden="true"]'),
      )
      if (!control) continue
      control.focus({ preventScroll: true })
      break
    }
  }
  function pop(event: PopStateEvent) {
    const position = Number(event.state?.position ?? observed)
    const previousPosition = observed
    observed = position
    if (pending) {
      pauseRouter()
      const finished = pending
      pending = undefined
      setTimeout(finished.resolve, 0)
      return
    }
    const target = read()
    const samePage =
      window.location.pathname + window.location.search + window.location.hash ===
      router.currentRoute.value.fullPath
    if (!samePage) {
      routePop = true
      return
    }
    if (!target) return
    pauseRouter()
    restoring = true
    if (target.dialogs >= open.length && open.some((entry) => entry.guard?.() === false)) {
      restoring = false
      void travel(previousPosition).then(schedule)
      return
    }
    if (!closeTo(target.dialogs)) {
      restoring = false
      void travel(previousPosition).then(schedule)
      return
    }
    void apply(target).then(() => {
      // A confirmation or removed media source cannot be reconstructed safely.
      // Skip unavailable entries without retaining callbacks or detached components.
      if (open.length < target.dialogs) {
        const direction = position < previousPosition ? -1 : 1
        if (direction > 0 && !event.state?.forward) void travel(previousPosition)
        else history.go(direction)
      } else {
        clearTimeout(focusTimer)
        focusTimer = setTimeout(restoreViewFocus, 0)
      }
    })
  }
  installViewHistoryEvents()
  popHandlers.add(pop)
  const stopGuard = router.beforeEach(async (to, from) => {
    if (to.fullPath === from.fullPath) return
    restoring = true
    if (!closeTo(0)) {
      restoring = false
      return false
    }
    restoring = false
    // The browser has already selected the destination entry. Do not write source-view
    // changes into that entry while Vue Router is completing Back or Forward.
    if (routePop) return
    await nextTick()
    await flush()
    if (pending) await pending.promise
  })
  const stopAfter = router.afterEach((_to, _from, failure) => {
    routePop = false
    if (failure) return
    observed = Number(window.history.state?.position ?? observed)
    void apply(read()).then(() => replace(snapshot()))
  })
  if (read()) void apply(read())
  return {
    register(close: () => unknown, guard?: () => boolean, modal = true) {
      const entry = { close, guard, modal }
      open.push(entry)
      schedule()
      return () => remove(entry)
    },
    get count() {
      return open.length
    },
    get modalCount() {
      return open.filter((entry) => entry.modal).length
    },
    closeTop() {
      const entry = open.at(-1)
      if (!entry) return false
      if (entry.close() !== false) remove(entry)
      return true
    },
    track<T extends ViewState>(key: string, state: Ref<T>) {
      const initial = clone(state.value)
      const owner = {
        initial,
        read: () => clone(state.value),
        write: (value: ViewState) => {
          if (!equal(state.value, value)) state.value = clone(value) as T
        },
      }
      owners.set(key, owner)
      const target = desired ?? read()
      if (target && key in target.values) {
        // Data-dependent children can mount after the initial restoration has finished.
        const wasRestoring = restoring
        restoring = true
        owner.write(target.values[key]!)
        restoring = wasRestoring
      } else {
        // Defaults need no history data, even for a timeline with many media components.
        if (!read()) replace(snapshot())
      }
      const stop = watch(state, schedule, { deep: true, flush: 'sync' })
      return () => {
        stop()
        if (owners.get(key) === owner) owners.delete(key)
      }
    },
    dispose() {
      disposed = true
      clearTimeout(resumeTimer)
      clearTimeout(focusTimer)
      if (previousListening !== undefined) router.listening = previousListening
      popHandlers.delete(pop)
      stopGuard()
      stopAfter()
      owners.clear()
      open.splice(0)
      pending?.resolve()
    },
  }
}
