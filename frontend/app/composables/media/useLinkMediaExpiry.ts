import { offlineStatus } from '../../utils/offline-archive'

type ExpiringAsset = { url: string; expiresAt?: string }

/** Media, downloads, and card thumbnails share one deadline and three bounded refresh attempts. */
export function useLinkMediaExpiry(options: {
  key: MaybeRefOrGetter<string>
  assets: MaybeRefOrGetter<readonly ExpiringAsset[] | undefined>
  enabled: MaybeRefOrGetter<boolean>
  pending: MaybeRefOrGetter<boolean>
  refresh: () => Promise<unknown>
}) {
  const expired = ref(false)
  const visible = ref(typeof document !== 'undefined' && !document.hidden)
  const online = ref(typeof navigator !== 'undefined' && navigator.onLine)
  const active = computed(
    () => toValue(options.enabled) && visible.value && online.value && !offlineStatus.value.offline,
  )
  const snapshot = computed(() => {
    const resources = toValue(options.assets)
    if (!resources) return undefined
    const deadlines = resources
      .map((asset) => [asset.url, asset.expiresAt ? Date.parse(asset.expiresAt) : NaN] as const)
      .filter((entry) => Number.isFinite(entry[1]))
    return {
      signature: JSON.stringify(deadlines),
      deadline: deadlines.length ? Math.min(...deadlines.map((entry) => entry[1])) : undefined,
    }
  })
  let key: string | undefined
  let signature: string | undefined
  let deadline: number | undefined
  let attempts = 0
  let retryAt = 0
  let inFlight = false
  let disposed = false
  let timer: ReturnType<typeof setTimeout> | undefined
  const backoff = [15_000, 60_000]

  function clearTimer() {
    clearTimeout(timer)
    timer = undefined
  }
  function schedule() {
    clearTimer()
    if (disposed) return
    const now = Date.now()
    expired.value = deadline !== undefined && deadline <= now
    if (deadline === undefined) return
    if (!expired.value) {
      // Mark stale sources even offscreen; only network refreshes require an active preview.
      // A CDN copy can live longer than the browser's roughly 25-day timeout limit.
      timer = setTimeout(schedule, Math.min(deadline - now, 2_147_483_647))
      return
    }
    if (!active.value || inFlight || toValue(options.pending) || attempts >= 3) return
    // A resolve before expiry legitimately returns the old copy. Allow a small clock margin.
    const refreshAt = Math.max(deadline + 1_000, retryAt)
    if (refreshAt <= now) {
      void refresh()
      return
    }
    timer = setTimeout(schedule, Math.min(refreshAt - now, 2_147_483_647))
  }
  async function refresh() {
    if (
      disposed ||
      !active.value ||
      document.hidden ||
      !navigator.onLine ||
      toValue(options.pending)
    )
      return
    const requestedKey = key
    const requestedSignature = signature
    attempts++
    inFlight = true
    try {
      await options.refresh()
    } catch {
      // The query keeps its visible failure state and the original link stays usable.
    } finally {
      inFlight = false
      if (!disposed) {
        if (key === requestedKey && signature === requestedSignature)
          retryAt = Date.now() + (backoff[attempts - 1] ?? 60_000)
        schedule()
      }
    }
  }
  watch(
    [() => toValue(options.key), snapshot, active, () => toValue(options.pending)],
    ([nextKey, next]) => {
      if (nextKey !== key) {
        key = nextKey
        signature = undefined
        deadline = undefined
        attempts = retryAt = 0
      }
      // A failed refetch may clear query data. Keep its previous deadline and retry budget.
      if (next && next.signature !== signature) {
        signature = next.signature
        deadline = next.deadline
        attempts = retryAt = 0
      }
      schedule()
    },
    { immediate: true, flush: 'post' },
  )
  function visibility() {
    visible.value = !document.hidden
  }
  function connectivity() {
    online.value = navigator.onLine
  }
  onMounted(() => {
    document.addEventListener('visibilitychange', visibility)
    window.addEventListener('online', connectivity)
    window.addEventListener('offline', connectivity)
    visibility()
    connectivity()
  })
  onBeforeUnmount(() => {
    disposed = true
    clearTimer()
    document.removeEventListener('visibilitychange', visibility)
    window.removeEventListener('online', connectivity)
    window.removeEventListener('offline', connectivity)
  })
  return { expired }
}
