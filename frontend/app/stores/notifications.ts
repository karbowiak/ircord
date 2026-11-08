import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import type { Conversation } from '../../shared/types/chat'
import { addIgnore, hasIgnore, removeIgnore } from '../../shared/utils/ignore-command'
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
import {
  deviceModeKey,
  deviceStatus,
  legacyDeviceModeKey,
  pushPossible,
  sanitizeDeviceMode,
  sanitizeSetupRecord,
  setupRecordKey,
  type DeviceFacts,
  type DeviceMode,
  type SetupRecord,
} from '../../shared/notification-setup'
import { devicesQuery } from '../queries/devices'

const key = ['notification-settings']
export type { DeviceMode } from '../../shared/notification-setup'

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
    (value: NotificationSettings) => apiClient().saveNotifications(value),
    {
      delay: 400,
      onSaved: (_value, saved) => {
        queryCache.cancelQueries({ key })
        queryCache.setQueryData(key, saved)
      },
      onError: () => useChatStore().notify('Couldn’t save notification settings.'),
    },
  )
  /** Whether the real settings arrived; until then `settings` holds the defaults. */
  const loaded = computed(() => query.data.value !== undefined)
  /**
   * Saves are full replacements, so one built from the defaults would wipe the stored
   * settings. Refuses (and says so) until they have loaded.
   */
  function update(changes: Partial<NotificationSettings>) {
    if (!loaded.value) {
      useChatStore().notify('Your settings are still loading; try again in a moment.')
      return false
    }
    const next = sanitizeNotificationSettings({ ...settings.value, ...changes })
    queryCache.setQueryData(key, next)
    queue.push(next)
    return true
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
    const list = settings.value.ignore
    return update({
      ignore: hasIgnore(list, nick) ? removeIgnore(list, nick) : addIgnore(list, nick),
    })
  }
  const highlights = (text: string, you: Parameters<typeof isHighlight>[2]) =>
    isHighlight(settings.value, text, you)

  // ---- This device ----------------------------------------------------------------------------

  const auth = useAuthStore()
  const accountId = auth.account?.id ?? 'signed-out'
  const supported = {
    notifications: typeof Notification !== 'undefined',
    push: 'serviceWorker' in navigator && 'PushManager' in window,
  }
  /** iOS only allows notifications for sites added to the Home Screen. */
  const ios =
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  const standalone = () =>
    window.matchMedia('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
  /**
   * Opened as an installed app. Only the launch mode is detectable: a browser tab says nothing
   * about whether the app is also installed somewhere on this device.
   */
  const installed = ref(standalone())
  const needsInstall = computed(() => ios && !installed.value)
  const readPermission = (): NotificationPermission | 'unsupported' =>
    supported.notifications ? Notification.permission : 'unsupported'
  const permission = ref(readPermission())

  // The choice is kept per account on this device, so another account signing in here never
  // inherits (and silently re-registers) a push subscription. A choice saved before that
  // belongs to whoever used this browser; it is adopted once.
  const modeKey = deviceModeKey(accountId)
  /** The push endpoint this device last registered, so it can be removed after a revoke. */
  const endpointKey = `ircord.device.push-endpoint.${accountId}`
  let stored = readLocal(modeKey, sanitizeDeviceMode)
  if (stored === null && auth.account) {
    stored = readLocal(legacyDeviceModeKey, sanitizeDeviceMode)
    if (stored !== null) {
      writeLocal(modeKey, stored)
      try {
        localStorage.removeItem(legacyDeviceModeKey)
      } catch {
        // Storage unavailable: the per-account copy is what counts.
      }
    }
  }
  const mode = ref<DeviceMode>(stored ?? 'off')
  /** The mode was chosen deliberately here, so a stored Off is a refusal, not the default. */
  const chosen = ref(stored !== null)
  const busy = ref(false)

  /** Whether this server can send push: undefined until it answered. */
  const pushKey = useQuery({
    key: ['push-key'],
    query: () => apiClient().pushKey(),
    staleTime: Infinity,
    enabled: () => supported.push && !needsInstall.value,
  })
  const serverPush = computed(() =>
    pushKey.data.value ? !!pushKey.data.value.publicKey : undefined,
  )
  /**
   * The server's record of this device, for whether push is registered there. Its own entry
   * under the devices key: devices.updated still refreshes it, and the Devices list keeps
   * loading fresh when opened instead of showing this startup copy.
   */
  const devices = useQuery({ ...devicesQuery, key: [...devicesQuery.key, 'this-device'] })
  const serverRegistered = computed(() => {
    const current = devices.data.value?.devices?.find((d) => d.current)
    if (current) return current.push === 'registered'
    // No device list (it failed, or this server has none): the browser's own record is all
    // there is to go on.
    return devices.error.value ? browserSubscribed.value : undefined
  })
  /** The browser still holds a push subscription; undefined until checked. */
  const browserSubscribed = ref<boolean | undefined>()

  async function registration() {
    return navigator.serviceWorker.ready
  }
  async function checkSubscription() {
    if (!supported.push) return (browserSubscribed.value = false)
    try {
      const reg = await registration()
      browserSubscribed.value = !!(await reg.pushManager.getSubscription())
    } catch {
      browserSubscribed.value = undefined
    }
  }
  const facts = computed<DeviceFacts>(() => ({
    notifications: supported.notifications,
    push: supported.push,
    needsInstall: needsInstall.value,
    permission: permission.value,
    mode: mode.value,
    serverPush: serverPush.value,
    browserSubscribed: browserSubscribed.value,
    serverRegistered: serverRegistered.value,
  }))
  const status = computed(() => deviceStatus(facts.value))
  /** Push can be set up on this device right now. */
  const pushAvailable = computed(() => pushPossible(facts.value))

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
    browserSubscribed.value = true
    const value = subscription.toJSON()
    if (value.endpoint) writeLocal(endpointKey, value.endpoint)
    if (!value.endpoint || !value.keys?.auth || !value.keys.p256dh)
      throw new Error('The browser returned an incomplete push subscription.')
    await apiClient().subscribePush({
      subscription: {
        endpoint: value.endpoint,
        keys: { auth: value.keys.auth, p256dh: value.keys.p256dh },
        expirationTime: value.expirationTime ?? null,
      },
      device: deviceName(),
    })
  }
  /**
   * Removes this device's push registration from the server and the browser. A browser that
   * already dropped its subscription (permission revoked while closed) still has the endpoint
   * this device last registered, so the server's copy goes too.
   */
  async function unsubscribe() {
    const subscription = supported.push
      ? await (await registration()).pushManager.getSubscription().catch(() => null)
      : null
    const endpoint =
      subscription?.endpoint ?? readLocal(endpointKey, (v) => (typeof v === 'string' ? v : null))
    if (endpoint)
      await apiClient()
        .unsubscribePush(endpoint)
        .catch(() => {})
    await subscription?.unsubscribe().catch(() => {})
    writeLocal(endpointKey, null)
    browserSubscribed.value = false
  }
  /** Rereads the device's registration on the server, for the status summary. */
  const refreshDevices = () =>
    queryCache.invalidateQueries({ key: devicesQuery.key }).catch(() => {})
  /**
   * Switches this device between off, in-app only and push (works with IRCord closed). Asks for
   * permission only from here, which only a click reaches. Resolves whether it worked.
   */
  async function setMode(next: DeviceMode) {
    busy.value = true
    try {
      if (next !== 'off' && permission.value !== 'granted') {
        if (!supported.notifications) throw new Error('This browser can’t show notifications.')
        permission.value = await Notification.requestPermission()
        if (permission.value === 'default') throw new Error('Notifications weren’t allowed.')
        if (permission.value !== 'granted')
          throw new Error('Notifications are blocked for this site.')
      }
      if (next === 'push') await subscribe()
      else if (mode.value === 'push' || browserSubscribed.value) await unsubscribe()
      mode.value = next
      chosen.value = true
      writeLocal(modeKey, next)
      return true
    } catch (error) {
      useChatStore().notify(error instanceof Error ? error.message : errorText(error))
      return false
    } finally {
      busy.value = false
      if (supported.push) void checkSubscription()
      void refreshDevices()
    }
  }
  /** The first-use prompt's memory for this account on this device. */
  const setupRecord = ref(readLocal(setupRecordKey(accountId), sanitizeSetupRecord))
  function rememberSetup(value: SetupRecord) {
    setupRecord.value = value
    writeLocal(setupRecordKey(accountId), value)
  }
  /** The one-step setup: push where this device and server can, otherwise while open. */
  async function enable() {
    const ok = await setMode(pushAvailable.value ? 'push' : 'open')
    if (ok) rememberSetup('enabled')
    return ok
  }
  /** Re-registers a mode whose permission or push registration was lost. */
  async function repair() {
    const ok = await setMode(mode.value === 'off' ? 'push' : mode.value)
    if (ok) rememberSetup('enabled')
    return ok
  }

  /** Sends a test push to this device only, and says what happened. */
  async function test(): Promise<{ ok: boolean; message: string }> {
    try {
      await apiClient().testPush()
      return { ok: true, message: 'Sent. It should arrive on this device in a few seconds.' }
    } catch (error) {
      if (errorStatus(error) === 409) {
        void refreshDevices()
        return { ok: false, message: 'This device isn’t registered for push. Repair it above.' }
      }
      return { ok: false, message: errorText(error) }
    }
  }

  /**
   * Permission, the launch mode and the push subscription change outside the page (browser or OS
   * settings, installing the app). Reread them whenever the page comes back into view.
   */
  let refreshing: Promise<void> | undefined
  function refresh() {
    refreshing ??= (async () => {
      installed.value = standalone()
      permission.value = readPermission()
      await dropIfRevoked()
      if (supported.push) await checkSubscription()
      await refreshDevices()
    })().finally(() => (refreshing = undefined))
    return refreshing
  }
  /**
   * Permission revoked (often in OS settings while IRCord was closed): drop the server's
   * registration so no device list shows push as active, and nothing is pushed to it.
   */
  async function dropIfRevoked() {
    if (permission.value !== 'denied') return
    const registered = readLocal(endpointKey, (v) => typeof v === 'string')
    if (mode.value === 'push' || browserSubscribed.value || registered)
      await unsubscribe().catch(() => {})
  }
  // Coming back from browser or OS settings, or switching apps on a phone, makes the page
  // visible again; that also covers installed apps.
  const onVisible = () => {
    if (document.visibilityState === 'visible') void refresh()
  }
  document.addEventListener('visibilitychange', onVisible)
  const display = window.matchMedia('(display-mode: standalone)')
  const relaunched = () => void refresh()
  display.addEventListener?.('change', relaunched)
  void navigator.permissions
    ?.query({ name: 'notifications' as PermissionName })
    .then((p) => p.addEventListener('change', relaunched))
    .catch(() => {})

  // Push subscriptions can be renewed or revoked by the browser; keep the server in step.
  if (mode.value === 'push' && supported.push && permission.value === 'granted')
    void subscribe()
      .catch(() => {})
      .finally(() => void checkSubscription())
  else
    void dropIfRevoked().then(() => {
      if (supported.push) void checkSubscription()
      void refreshDevices()
    })

  /**
   * The service worker reports a renewed subscription. Re-register without asking: only a
   * click may bring up the permission prompt.
   */
  function resubscribe() {
    if (mode.value !== 'push' || permission.value !== 'granted' || !supported.push) return
    void subscribe()
      .catch(() => {})
      .finally(() => {
        void checkSubscription()
        void refreshDevices()
      })
  }

  return {
    query,
    settings,
    loaded,
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
    chosen,
    busy,
    status,
    facts,
    pushAvailable,
    serverPush,
    setMode,
    enable,
    repair,
    refresh,
    resubscribe,
    setupRecord,
    rememberSetup,
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
