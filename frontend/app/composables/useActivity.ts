import { formFactor } from '../../shared/utils/activity'

/** Renew an active report this often; the server lets one lapse after 60 seconds. */
export const ACTIVITY_RENEW_MS = 25_000
/** A focused tab without any input for this long counts as left alone, not read. */
export const ACTIVITY_IDLE_MS = 10 * 60_000
/** How often the tab re-evaluates idleness and renewal. */
const CHECK_MS = 5_000

/**
 * Tells the server whether this tab is in use, so it can route alerts across the account's
 * devices (#84): while any tab is active nothing interrupts on any device; once none is, the
 * most recently active device is alerted first and the others later.
 *
 * Active means visible, focused and touched in the last ten minutes. Reading counts: scrolling,
 * pointer and key input all reset the idle clock, and nothing about them is sent — only the
 * active flag, the tab ID and the device's form factor. Signals and their limits:
 * - focus/blur and visibility cover switching apps or tabs, minimising and most screen locks;
 * - pagehide sends a final inactive report (best effort; a crash or killed app sends nothing,
 *   and its lease simply lapses within a minute);
 * - a desktop locked without hiding the page looks focused until the idle timeout;
 * - a suspended mobile app stops renewing and lapses like a crash.
 * A reconnecting socket is not activity, and focusing the app never marks anything read.
 */
export function useActivity() {
  const chat = useChatStore()
  const notifications = useNotificationStore()
  const device = formFactor()
  /** This browser shows page notifications, so an open page can be alerted instead of push. */
  const canAlert = () => notifications.mode === 'open' && notifications.permission === 'granted'
  let lastInput = Date.now()
  let lastSent = 0
  let reported: boolean | undefined
  const visibility = useDocumentVisibility()

  const present = () =>
    visibility.value === 'visible' &&
    document.hasFocus() &&
    Date.now() - lastInput < ACTIVITY_IDLE_MS

  function send(active: boolean, keepalive = false) {
    reported = active
    lastSent = Date.now()
    void apiClient()
      .reportActivity(
        { tab: chat.tabId, active, formFactor: device, canAlert: canAlert() },
        keepalive,
      )
      .catch(() => {})
  }
  function evaluate() {
    if (!chat.routing) return
    const active = present()
    if (active !== reported || (active && Date.now() - lastSent >= ACTIVITY_RENEW_MS)) send(active)
  }
  function input() {
    lastInput = Date.now()
    if (reported !== true) evaluate()
  }
  function leave() {
    if (chat.routing && reported !== false) send(false, true)
  }

  useEventListener(window, ['pointerdown', 'keydown', 'wheel', 'touchstart', 'scroll'], input, {
    passive: true,
    capture: true,
  })
  useEventListener(window, ['focus', 'blur'], evaluate)
  watch(visibility, evaluate, { flush: 'sync' })
  useEventListener(window, 'pagehide', leave)
  // Keep the existing activity-lease cadence; this does not fetch chat state.
  useIntervalFn(evaluate, CHECK_MS)
  // Turning page notifications on or off, or losing permission, changes how this device can be
  // reached: tell the server straight away.
  watch(
    () => [notifications.mode, notifications.permission] as const,
    () => {
      if (chat.routing && reported !== undefined) send(present())
    },
  )
  // A new socket (after a restart, say) has no lease for this tab yet: report straight away.
  watch(
    () => [chat.routing, chat.status] as const,
    ([routing, status]) => {
      if (routing && status === 'connected') {
        reported = undefined
        evaluate()
      }
    },
    { immediate: true },
  )
  onBeforeUnmount(leave)
}
