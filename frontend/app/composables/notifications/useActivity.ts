import { formFactor } from '../../../shared/utils/activity'

/** Renew before the server's 60-second activity lease expires. */
export const ACTIVITY_RENEW_MS = 25_000
const CHECK_MS = 5_000
const ENGAGEMENT_COALESCE_MS = 1_000

/** Reports presence and genuine engagement without transmitting input details. */
export function useActivity(viewedConversation: () => string | undefined) {
  const chat = useChatStore()
  const notifications = useNotificationStore()
  const device = formFactor()
  const visibility = useDocumentVisibility()
  const present = () => visibility.value === 'visible' && document.hasFocus()
  const canAlert = () => notifications.mode === 'open' && notifications.permission === 'granted'
  let sequence = 0
  let lastSent = 0
  let lastEngagement = 0
  let reported: boolean | undefined
  let engagementPending = true
  let pointerActive = false
  let recentInput = 0
  let pending: ReturnType<typeof setTimeout> | undefined

  function send(signal: 'engagement' | 'renewal' | 'inactive', keepalive = false) {
    const connectionId = chat.routingSnapshot?.connectionId
    if (!chat.routing || !connectionId) return
    const active = signal !== 'inactive'
    reported = active
    lastSent = Date.now()
    if (signal === 'engagement') lastEngagement = lastSent
    void apiClient()
      .reportActivity(
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
      .catch(() => {})
  }
  function engage() {
    if (!present()) return
    if (!chat.routingSnapshot?.connectionId) {
      engagementPending = true
      return
    }
    engagementPending = false
    if (Date.now() - lastEngagement >= ENGAGEMENT_COALESCE_MS) {
      clearTimeout(pending)
      pending = undefined
      send('engagement')
    } else if (!pending) {
      pending = setTimeout(
        () => {
          pending = undefined
          if (present()) send('engagement')
        },
        ENGAGEMENT_COALESCE_MS - (Date.now() - lastEngagement),
      )
    }
  }
  function leave() {
    clearTimeout(pending)
    pending = undefined
    pointerActive = false
    engagementPending = false
    if (reported !== false) send('inactive', true)
    reported = false
  }
  function evaluate() {
    if (!present()) return leave()
    if (reported !== true) return engage()
    if (Date.now() - lastSent >= ACTIVITY_RENEW_MS) send('renewal')
  }

  useEventListener(
    window,
    ['pointerdown', 'keydown', 'wheel', 'touchstart', 'scroll'],
    (event) => {
      if (!event.isTrusted) return
      if (event.type === 'scroll') {
        // Browsers also emit trusted scroll events for layout and programmatic scrolling.
        if (!pointerActive && Date.now() - recentInput > ENGAGEMENT_COALESCE_MS) return
      } else {
        recentInput = Date.now()
        if (event.type === 'pointerdown' || event.type === 'touchstart') pointerActive = true
      }
      engage()
    },
    { passive: true, capture: true },
  )
  useEventListener(window, ['pointerup', 'pointercancel', 'touchend', 'touchcancel'], () => {
    pointerActive = false
  }, { passive: true, capture: true })
  useEventListener(window, 'focus', engage)
  useEventListener(window, 'blur', leave)
  watch(visibility, evaluate, { flush: 'sync' })
  useEventListener(window, 'pagehide', leave)
  useIntervalFn(evaluate, CHECK_MS)
  // A route or overlay can change without user input. Update the viewed conversation without
  // letting that change take the focal role from a more recently engaged device.
  watch(viewedConversation, () => {
    if (reported && present()) send('renewal')
  })
  watch(
    () => [notifications.mode, notifications.permission] as const,
    () => {
      if (reported !== undefined) send(present() ? 'renewal' : 'inactive')
    },
  )
  watch(
    () => chat.routingSnapshot?.connectionId,
    (connectionId) => {
      if (connectionId) {
        if (present()) {
          if (engagementPending || reported !== true) {
            lastEngagement = 0
            engage()
          } else send('renewal')
        } else send('inactive')
      }
    },
    { immediate: true },
  )
  onBeforeUnmount(leave)
}
