import { alertText, type NotificationAlert } from '../../shared/utils/activity'
import { claimNotification } from '../utils/notification-ledger'

/**
 * Alerts for the open app: in-page notifications when this device is set to “while open”
 * (push devices get them from the service worker instead), the unread count in the tab title
 * and app icon badge, and opening conversations from notification taps. Call once from the
 * workspace.
 *
 * When the server routes alerts (#84), it decides which device interrupts and when: this tab
 * only sounds and shows what arrives as notification.alert, and reports its activity
 * (useActivity).
 */
export function useNotifications(viewedConversation: () => string | undefined) {
  const { t, n } = useI18n()
  const chat = useChatStore()
  const notifications = useNotificationStore()

  /** Mentions plus unread direct messages, ignoring muted conversations. */
  const attention = computed(
    () =>
      chat.data?.conversations.reduce((sum, c) => {
        if (notifications.level(c) === 'none') return sum
        return sum + (c.kind === 'direct' ? c.unreadCount : c.mentionCount)
      }, 0) ?? 0,
  )
  watch(
    attention,
    (count) => {
      const nav = navigator as Navigator & {
        setAppBadge?: (n: number) => Promise<void>
        clearAppBadge?: () => Promise<void>
      }
      void (count ? nav.setAppBadge?.(count) : nav.clearAppBadge?.())?.catch(() => {})
    },
    { immediate: true },
  )

  const sound = useAlertSound()
  const activeWindow = () => document.visibilityState === 'visible' && document.hasFocus()
  const viewing = (alert: NotificationAlert) =>
    activeWindow() && viewedConversation() === alert.conversationId
  async function playSound(alert: NotificationAlert) {
    if (!alert.sound || viewing(alert)) return
    if (!(await claimNotification(`${alert.generation}:${alert.alertId}:sound`))) return
    if (!viewing(alert)) sound.play(alert.kind, true)
  }
  /** The server chooses sound or OS presentation and resolves the device's preferences. */
  async function alerted(alert: NotificationAlert) {
    const c = chat.data?.conversations.find((x) => x.id === alert.conversationId)
    if (alert.delivery === 'sound') {
      await playSound(alert)
      return
    }
    // A queued alert can arrive after the user has returned, before the activity report arrives.
    if (activeWindow()) return void (await playSound(alert))
    if (notifications.mode !== 'open' || notifications.permission !== 'granted') return
    if (!(await claimNotification(`${alert.generation}:${alert.alertId}:os`))) return
    if (activeWindow()) return void (await playSound(alert))
    const { title, body } = alertText(alert, (key, values, plural) => {
      const formatted = Object.fromEntries(
        Object.entries(values).map(([name, value]) => [
          name,
          typeof value === 'number' ? n(value) : value,
        ]),
      )
      return plural === undefined ? t(key, formatted) : t(key, formatted, plural)
    })
    const sender =
      c?.kind === 'direct' ? chat.member(c.memberIds.find((id) => id !== 'you') ?? '') : undefined
    const notification = new Notification(alert.preview ? title : 'IRCord', {
      body: alert.preview ? body.slice(0, 200) : '',
      icon: sender?.avatarUrl ?? '/icons/icon-192.png',
      tag: alert.conversationId,
      silent: !alert.sound,
    })
    notification.onclick = () => {
      window.focus()
      chat.activate(alert.conversationId)
      notification.close()
    }
  }
  const stopAlerts = chat.onAlert(alerted)

  // Notification taps arrive from the service worker.
  function fromWorker(event: MessageEvent) {
    if (event.data?.type === 'open-conversation') chat.activate(event.data.conversationId)
    if (event.data?.type === 'push-subscription-changed') notifications.resubscribe()
  }
  useEventListener(navigator.serviceWorker, 'message', fromWorker)
  // Cold start from a notification: /?c=<conversationId>.
  const requested = new URL(location.href).searchParams.get('c')
  if (requested) {
    const stopWaiting = watch(
      () => chat.data,
      (data) => {
        if (!data) return
        chat.activate(requested)
        history.replaceState(null, '', location.pathname)
        queueMicrotask(() => stopWaiting())
      },
      { immediate: true },
    )
  }
  onBeforeUnmount(() => {
    stopAlerts()
  })

  return { attention }
}
