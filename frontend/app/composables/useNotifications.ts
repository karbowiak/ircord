import type { Message } from '../../shared/types/chat'
import { notificationFor } from '../../shared/notifications'
import { channelTarget, stripFormatting } from '../../shared/utils/irc'

/**
 * Alerts for the open app: in-page notifications when this device is set to “while open”
 * (push devices get them from the service worker instead), the unread count in the tab title
 * and app icon badge, and opening conversations from notification taps. Call once from the
 * workspace.
 */
export function useNotifications() {
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
  function incoming(message: Message) {
    const c = chat.data?.conversations.find((x) => x.id === message.conversationId)
    if (!c) return
    const sender = chat.member(message.senderId)
    // Rules include levels, the ignore list and quiet hours.
    const kind = notificationFor(
      notifications.settings,
      c,
      message,
      chat.networkOf(c)?.nick,
      sender?.nick,
      new Date(),
      sender,
    )
    if (!kind) return
    // Only interrupt when the message isn't already in front of the user.
    const inFront =
      document.visibilityState === 'visible' && document.hasFocus() && chat.activeId === c.id
    if (inFront) return
    sound.play(kind)
    if (notifications.mode !== 'open' || notifications.permission !== 'granted') return
    const where = c.kind === 'channel' ? ` in ${channelTarget(c)}` : ''
    const notification = new Notification(`${sender?.nick ?? 'Someone'}${where}`, {
      body: message.gif ? 'Sent a GIF' : stripFormatting(message.text).slice(0, 200),
      icon: sender?.avatarUrl ?? '/icons/icon-192.png',
      tag: c.id,
      silent: sound.mode.value !== 'off',
    })
    notification.onclick = () => {
      window.focus()
      chat.activate(c.id)
      notification.close()
    }
  }
  const stop = chat.onIncoming(incoming)

  // Notification taps arrive from the service worker.
  function fromWorker(event: MessageEvent) {
    if (event.data?.type === 'open-conversation') chat.activate(event.data.conversationId)
    if (event.data?.type === 'push-subscription-changed' && notifications.mode === 'push')
      void notifications.setMode('push')
  }
  navigator.serviceWorker?.addEventListener('message', fromWorker)
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
    stop()
    navigator.serviceWorker?.removeEventListener('message', fromWorker)
  })

  return { attention }
}
