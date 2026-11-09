/**
 * This device's link to IRCord, for the server page: round trips to the API over the last half
 * hour, how far this device's clock is from the server's, and the HTTP protocol in use. Samples
 * are taken every 20 seconds while the app is visible; the workspace starts the sampler.
 */
export interface LinkSample {
  at: string
  ms: number
}
const INTERVAL = 20_000
const WINDOW = 30 * 60_000
const samples = ref<LinkSample[]>([])
/** Server clock minus this device's clock, in milliseconds. */
const clockOffset = ref<number | null>(null)
const protocol = ref('')
const failing = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
let users = 0
let inFlight = false

async function sample() {
  if (inFlight || document.hidden) return
  inFlight = true
  const sent = Date.now()
  const started = performance.now()
  try {
    const { serverTime } = await apiClient().ping()
    const rtt = performance.now() - started
    const now = Date.now()
    // The server read its clock about halfway through the round trip.
    clockOffset.value = new Date(serverTime).getTime() - (sent + rtt / 2)
    failing.value = false
    samples.value = [
      ...samples.value.filter((s) => now - new Date(s.at).getTime() <= WINDOW),
      { at: new Date(now).toISOString(), ms: Math.round(rtt * 10) / 10 },
    ]
    // Read once: each later ping would only add to the browser's timing buffer.
    if (!protocol.value) {
      const entry = performance
        .getEntriesByType('resource')
        .findLast((e) => e.name.includes('/api/v1/ping')) as PerformanceResourceTiming | undefined
      protocol.value = entry?.nextHopProtocol ?? ''
    }
  } catch {
    failing.value = true
  } finally {
    inFlight = false
  }
}
const visible = () => !document.hidden && void sample()

/** Starts sampling for as long as one caller (the workspace) holds it. */
export function useClientLinkSampler() {
  onMounted(() => {
    if (users++) return
    void sample()
    timer = setInterval(() => void sample(), INTERVAL)
    document.addEventListener('visibilitychange', visible)
  })
  onBeforeUnmount(() => {
    if (--users) return
    clearInterval(timer)
    document.removeEventListener('visibilitychange', visible)
    samples.value = []
    clockOffset.value = null
  })
}

export function useClientLink() {
  return {
    samples: readonly(samples),
    clockOffset: readonly(clockOffset),
    protocol: readonly(protocol),
    failing: readonly(failing),
    intervalSeconds: INTERVAL / 1000,
    windowSeconds: WINDOW / 1000,
  }
}
