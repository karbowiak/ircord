import { formFactor } from '../../../shared/utils/activity'

/** Renew before the server's 60-second activity lease expires. */
export const ACTIVITY_RENEW_MS = 25_000
const ACTIVITY_REPORT_MS = 10_000
const INPUT_SCROLL_WINDOW_MS = 1_000

/** Reports presence and genuine engagement without transmitting input details. */
export function useActivity(viewedConversation: () => string | undefined) {
  const chat = useChatStore()
  const notifications = useNotificationStore()
  const device = formFactor()
  const visibility = useDocumentVisibility()
  const present = () =>
    !disposed && !pageHidden && visibility.value === 'visible' && document.hasFocus()
  const connection = () => (chat.routing ? chat.routingSnapshot?.connectionId : undefined)
  const canAlert = () => notifications.mode === 'open' && notifications.permission === 'granted'
  let sequence = 0
  let lastSent: number | undefined
  let reported: boolean | undefined
  let engagementPending = true
  let statePending = false
  let pointerActive = false
  let recentInput = -Infinity
  let disposed = false
  let pageHidden = false
  let generation = 0
  let sending = false
  let pending: ReturnType<typeof setTimeout> | undefined
  let pendingAt: number | undefined

  function report(
    signal: 'engagement' | 'renewal' | 'inactive',
    connectionId: string,
    keepalive = false,
  ) {
    const active = signal !== 'inactive'
    return apiClient().reportActivity(
      {
        tab: chat.tabId,
        incarnation: chat.incarnation,
        connectionId,
        sequence: ++sequence,
        signal,
        active,
        viewedConversationId: active ? viewedConversation() : undefined,
        formFactor: device,
        canAlert: canAlert(),
        permission: notifications.permission,
      },
      keepalive,
    )
  }
  function cancelPending() {
    clearTimeout(pending)
    pending = undefined
    pendingAt = undefined
  }
  function schedule() {
    if (!present() || !connection() || sending) return
    const deadline =
      lastSent === undefined
        ? Date.now()
        : lastSent + (engagementPending || statePending ? ACTIVITY_REPORT_MS : ACTIVITY_RENEW_MS)
    if (pendingAt === deadline) return
    cancelPending()
    const scheduledGeneration = generation
    pendingAt = deadline
    pending = setTimeout(
      () => {
        if (scheduledGeneration !== generation || disposed) return
        pending = undefined
        pendingAt = undefined
        if (!present()) return leave()
        sendActive()
      },
      Math.max(0, deadline - Date.now()),
    )
  }
  function sendActive() {
    const connectionId = connection()
    if (!present() || !connectionId || sending) return
    cancelPending()
    const signal = engagementPending ? 'engagement' : 'renewal'
    const sentGeneration = generation
    engagementPending = false
    statePending = false
    reported = true
    lastSent = Date.now()
    sending = true
    void report(signal, connectionId)
      .catch(() => {
        if (sentGeneration !== generation || !present()) return
        // Retry the newest state on the same bounded cadence, including failed engagement.
        statePending = true
        engagementPending ||= signal === 'engagement'
      })
      .finally(() => {
        if (sentGeneration !== generation) return
        sending = false
        schedule()
      })
  }
  function engage() {
    if (!present()) return
    // Input only changes local flags. The shared timer sends the latest aggregate state.
    engagementPending = true
    schedule()
  }
  function leave() {
    cancelPending()
    generation++
    sending = false
    pointerActive = false
    recentInput = -Infinity
    engagementPending = false
    statePending = false
    const connectionId = connection()
    if (!disposed && reported !== false && connectionId)
      void report('inactive', connectionId, true).catch(() => {})
    reported = false
  }
  function evaluate() {
    if (!present()) return leave()
    // Returning to the foreground must re-establish a released lease; renewal cannot do so.
    if (reported !== true) return engage()
    schedule()
  }

  useEventListener(
    window,
    ['pointerdown', 'keydown', 'wheel', 'touchstart', 'scroll'],
    (event) => {
      if (!event.isTrusted) return
      if (event.type === 'scroll') {
        // Browsers also emit trusted scroll events for layout and programmatic scrolling.
        if (!pointerActive && Date.now() - recentInput > INPUT_SCROLL_WINDOW_MS) return
      } else {
        recentInput = Date.now()
        if (event.type === 'pointerdown' || event.type === 'touchstart') pointerActive = true
      }
      engage()
    },
    { passive: true, capture: true },
  )
  useEventListener(
    window,
    ['pointerup', 'pointercancel', 'touchend', 'touchcancel'],
    () => {
      pointerActive = false
    },
    { passive: true, capture: true },
  )
  useEventListener(window, 'focus', engage)
  useEventListener(window, 'blur', leave)
  watch(visibility, evaluate, { flush: 'sync' })
  useEventListener(window, 'pagehide', () => {
    pageHidden = true
    leave()
  })
  useEventListener(window, 'pageshow', () => {
    pageHidden = false
    evaluate()
  })
  // A route or overlay can change without user input. Update the viewed conversation without
  // letting that change take the focal role from a more recently engaged device.
  watch(viewedConversation, () => {
    statePending = true
    schedule()
  })
  watch(
    () => [notifications.mode, notifications.permission] as const,
    () => {
      statePending = true
      schedule()
    },
  )
  watch(
    [connection, () => chat.tabId, () => chat.incarnation],
    () => {
      cancelPending()
      generation++
      sending = false
      lastSent = undefined
      reported = undefined
      statePending = true
      if (!connection() || disposed) return
      // A fresh socket needs its binding immediately; renewal does not select the focal device.
      if (present()) sendActive()
      else leave()
    },
    { immediate: true, flush: 'sync' },
  )
  onBeforeUnmount(() => {
    leave()
    disposed = true
  })
}
