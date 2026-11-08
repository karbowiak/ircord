/**
 * What notifications on this device need, and when to offer setting them up (#189). Pure, so
 * the rules can be tested without a browser; the notification store feeds in what it observed.
 */

/** What this device does with notifications. Permission and push are per device. */
export type DeviceMode = 'off' | 'open' | 'push'

export interface DeviceFacts {
  /** The browser can show notifications at all. */
  notifications: boolean
  /** The browser supports Web Push (service worker and PushManager). */
  push: boolean
  /** iPhone/iPad outside a Home Screen app: notifications need the installed app first. */
  needsInstall: boolean
  permission: NotificationPermission | 'unsupported'
  mode: DeviceMode
  /** Whether this server can send push: unknown until it answered. */
  serverPush: boolean | undefined
  /** The browser holds a push subscription; unknown until checked. */
  browserSubscribed: boolean | undefined
  /** The server has this device's push registration; unknown until the device list loaded. */
  serverRegistered: boolean | undefined
}

/**
 * This device's notification state, as Settings summarises it:
 * - `install`: add to the Home Screen and open from there first.
 * - `unsupported`: this browser can't show notifications.
 * - `blocked`: the browser or OS denies permission; only its settings can change that.
 * - `off`: nothing on this device.
 * - `permission`: a mode is chosen but permission was reset; asking again needs a click.
 * - `open`: alerts while IRCord is open.
 * - `checking`: push is chosen and its registration is still being confirmed.
 * - `push`: push is chosen, permitted, subscribed in the browser and registered on the server.
 * - `repair`: push is chosen and permitted, but the browser or server lost the registration.
 * - `server-unavailable`: push is chosen but this server has no Web Push configured.
 */
export type DeviceStatus =
  | 'install'
  | 'unsupported'
  | 'blocked'
  | 'off'
  | 'permission'
  | 'open'
  | 'checking'
  | 'push'
  | 'repair'
  | 'server-unavailable'

export function deviceStatus(f: DeviceFacts): DeviceStatus {
  if (f.needsInstall) return 'install'
  if (!f.notifications || f.permission === 'unsupported') return 'unsupported'
  if (f.permission === 'denied') return 'blocked'
  if (f.mode === 'off') return 'off'
  if (f.permission !== 'granted') return 'permission'
  if (f.mode === 'open') return 'open'
  if (!f.push) return 'repair'
  if (f.serverPush === false) return 'server-unavailable'
  if (f.browserSubscribed === false || f.serverRegistered === false) return 'repair'
  if (
    f.serverPush === undefined ||
    f.browserSubscribed === undefined ||
    f.serverRegistered === undefined
  )
    return 'checking'
  return 'push'
}

/** Push can be set up here: the browser supports it and the server confirmed it can send. */
export const pushPossible = (
  f: Pick<DeviceFacts, 'notifications' | 'push' | 'needsInstall' | 'serverPush'>,
) => f.notifications && f.push && !f.needsInstall && f.serverPush === true

/**
 * What the first-use prompt remembers for one account on one device (browser storage is per
 * browser and per installed app, so a new phone, or the Home Screen app after the Safari tab,
 * starts fresh):
 * - `not-now`: dismissed; never offered again here. Settings still offers setup.
 * - `enabled`: set up from the prompt or Settings; only a broken registration is offered again.
 */
export type SetupRecord = 'not-now' | 'enabled'
export const setupRecordKey = (account: string) => `ircord.notification-setup.${account}`
export const sanitizeSetupRecord = (value: unknown): SetupRecord | null =>
  value === 'not-now' || value === 'enabled' ? value : null

/** Per account on this device, so a different account never inherits a push choice. */
export const deviceModeKey = (account: string) => `ircord.device.notifications.${account}`
/** Before #189 the choice was kept once per browser, whoever was signed in. */
export const legacyDeviceModeKey = 'ircord.device.notifications'
export const sanitizeDeviceMode = (value: unknown): DeviceMode | null =>
  value === 'off' || value === 'open' || value === 'push' ? value : null

/** The first-use prompt's offer: turn notifications on, finish a broken setup, or install first. */
export type SetupOffer = 'enable' | 'repair' | 'install'

export interface OfferInput {
  facts: DeviceFacts
  /** A phone or tablet, or IRCord opened as an installed app. */
  eligible: boolean
  /** The device's mode was chosen deliberately (stored), not the default Off. */
  chosen: boolean
  record: SetupRecord | null
  online: boolean
}

/**
 * Whether and what to offer, following #189's situation table. Never offers on uncertain
 * capability (still checking, offline, server push unknown or unavailable), after a refusal
 * (permission denied, a deliberate Off, Not now), or when notifications already work.
 */
export function setupOffer({
  facts,
  eligible,
  chosen,
  record,
  online,
}: OfferInput): SetupOffer | null {
  if (!eligible || !online || record === 'not-now') return null
  const status = deviceStatus(facts)
  if (status === 'install') return record ? null : 'install'
  if (status === 'repair' || status === 'permission')
    return facts.mode === 'open' || pushPossible(facts) ? 'repair' : null
  if (status !== 'off' || record || (chosen && facts.mode === 'off')) return null
  return pushPossible(facts) ? 'enable' : null
}
