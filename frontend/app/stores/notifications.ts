import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import type { Conversation } from '../../shared/types/chat'
import {
  defaultNotificationSettings,
  isHighlight,
  isIgnored,
  levelFor,
  sanitizeNotificationSettings,
  type LevelTarget,
  type NotificationSettings,
  type NotifyLevel,
} from '../../shared/notifications'

const key = ['notification-settings']
const deviceKey = 'ircord.device.notifications'
/** What this device does with notifications. Permission and push are per device. */
export type DeviceMode = 'off' | 'open' | 'push'

const base64ToBytes = (base64: string) => {
  const padded = (base64 + '='.repeat((4 - (base64.length % 4)) % 4))
    .replace(/-/g, '+')
    .replace(/_/g, '/')
  return Uint8Array.from(atob(padded), (c) => c.charCodeAt(0))
}

/**
 * Notification rules (levels, highlight words, ignore list) live on the backend, which sends
 * push notifications while no IRCord window is open. This store edits them and manages this
 * device's permission and push subscription.
 */
export const useNotificationStore = defineStore('notifications', () => {
  const queryCache = useQueryCache()
  const query = useQuery({
    key,
    query: () => apiClient().notificationSettings(),
    staleTime: 60_000,
    refetchOnWindowFocus: true,
  })
  const settings = computed(() =>
    sanitizeNotificationSettings(query.data.value ?? defaultNotificationSettings),
  )

  const queue = createLatestQueue(
    (value: NotificationSettings) =>
      apiClient().saveNotifications(value),
    {
      delay: 400,
      onSaved: (_value, saved) => {
        queryCache.cancelQueries({ key })
        queryCache.setQueryData(key, saved)
      },
      onError: () => useChatStore().notify('Couldn’t save notification settings.'),
    },
  )
  function update(changes: Partial<NotificationSettings>) {
    const next = sanitizeNotificationSettings({ ...settings.value, ...changes })
    queryCache.setQueryData(key, next)
    queue.push(next)
  }
  const level = (c: LevelTarget) => levelFor(settings.value, c)
  /** Your own level for a direct message. Channel levels are channel settings. */
  function setLevel(c: Pick<Conversation, 'id' | 'kind'>, value: NotifyLevel | null) {
    if (c.kind !== 'direct') return
    const levels = { ...settings.value.levels }
    if (value === null) delete levels[c.id]
    else levels[c.id] = value
    update({ levels })
  }
  const ignored = (nick: string | undefined) => {
    const member = useChatStore().memberByNick(nick ?? '')
    return isIgnored(settings.value, nick, member?.username, member?.host)
  }
  function toggleIgnore(nick: string) {
    const exists = settings.value.ignore.some((mask) => mask.toLowerCase() === nick.toLowerCase())
    update({
      ignore: exists
        ? settings.value.ignore.filter((mask) => mask.toLowerCase() !== nick.toLowerCase())
        : [...settings.value.ignore, nick],
    })
  }
  const highlights = (text: string, nick: string | undefined) =>
    isHighlight(settings.value, text, nick)

  // ---- This device ----------------------------------------------------------------------------

  const supported = {
    notifications: typeof Notification !== 'undefined',
    push: 'serviceWorker' in navigator && 'PushManager' in window,
  }
  /** iOS only allows notifications for sites added to the Home Screen. */
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  const installed =
    window.matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
  const needsInstall = ios && !installed
  const permission = ref<NotificationPermission | 'unsupported'>(
    supported.notifications ? Notification.permission : 'unsupported',
  )
  const mode = ref<DeviceMode>(
    readLocal(deviceKey, (v) => (v === 'open' || v === 'push' ? v : 'off')),
  )
  const busy = ref(false)

  async function registration() {
    return navigator.serviceWorker.ready
  }
  async function subscribe() {
    const { publicKey } = await apiClient().pushKey()
    if (!publicKey) throw new Error('Web Push isn’t configured on this server.')
    const reg = await registration()
    const existing = await reg.pushManager.getSubscription()
    const subscription =
      existing ??
      (await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: base64ToBytes(publicKey),
      }))
    const value = subscription.toJSON()
    if (!value.endpoint || !value.keys?.auth || !value.keys.p256dh) throw new Error('The browser returned an incomplete push subscription.')
    await apiClient().subscribePush({ subscription: { endpoint: value.endpoint, keys: { auth: value.keys.auth, p256dh: value.keys.p256dh }, expirationTime: value.expirationTime ?? null }, device: deviceName() })
  }
  async function unsubscribe() {
    const subscription = await (
      await registration()
    ).pushManager
      .getSubscription()
      .catch(() => null)
    if (!subscription) return
    await apiClient().unsubscribePush(subscription.endpoint).catch(() => {})
    await subscription.unsubscribe().catch(() => {})
  }
  /** Switches this device between off, in-app only and push (works with IRCord closed). */
  async function setMode(next: DeviceMode) {
    busy.value = true
    try {
      if (next !== 'off' && permission.value !== 'granted') {
        if (!supported.notifications) throw new Error('This browser can’t show notifications.')
        permission.value = await Notification.requestPermission()
        if (permission.value !== 'granted')
          throw new Error('Notifications are blocked for this site.')
      }
      if (next === 'push') await subscribe()
      else if (mode.value === 'push') await unsubscribe()
      mode.value = next
      writeLocal(deviceKey, next)
    } catch (error) {
      useChatStore().notify(error instanceof Error ? error.message : errorText(error))
    } finally {
      busy.value = false
    }
  }
  async function test() {
    try {
      await apiClient().testPush()
    } catch (error) {
      useChatStore().notify(errorText(error))
    }
  }
  // Push subscriptions can be renewed or revoked by the browser; keep the server in step.
  if (mode.value === 'push' && supported.push && permission.value === 'granted')
    void subscribe().catch(() => {})
  if (mode.value !== 'off' && permission.value === 'denied') mode.value = 'off'

  return {
    query,
    settings,
    update,
    level,
    setLevel,
    ignored,
    toggleIgnore,
    highlights,
    supported,
    needsInstall,
    installed,
    permission,
    mode,
    busy,
    setMode,
    test,
  }
})

function deviceName() {
  const ua = navigator.userAgent
  const os = /Android/.test(ua)
    ? 'Android'
    : /iPhone|iPad|iPod/.test(ua)
      ? 'iOS'
      : /Mac/.test(ua)
        ? 'macOS'
        : /Windows/.test(ua)
          ? 'Windows'
          : /Linux/.test(ua)
            ? 'Linux'
            : 'Device'
  const browser = /Edg\//.test(ua)
    ? 'Edge'
    : /Firefox\//.test(ua)
      ? 'Firefox'
      : /Chrome\//.test(ua)
        ? 'Chrome'
        : /Safari\//.test(ua)
          ? 'Safari'
          : ''
  return `${browser} on ${os}`.trim()
}

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useNotificationStore, import.meta.hot))
