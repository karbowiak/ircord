import type { components } from '../generated/api'

export type Device = components['schemas']['Device']

export type DeviceMessageKey =
  | 'unknownBrowser'
  | 'browserOnOs'
  | 'justNow'
  | 'lastSeenUnknown'
  | 'lastSeen'
  | 'activeNow'
  | 'connectedNow'
  | 'notificationsPush'
  | 'notificationsPushNotRegistered'
  | 'notificationsWhileOpen'
  | 'notificationsOff'
  | 'pushRegistered'
  | 'noPush'
export type DeviceTranslate = (
  key: DeviceMessageKey,
  values?: { browser?: string; os?: string; time?: string },
) => string
const label = (
  translate: DeviceTranslate | undefined,
  key: DeviceMessageKey,
  fallback: string,
  values?: { browser?: string; os?: string; time?: string },
) => translate?.(key, values) ?? fallback

/** "Firefox on Linux"; the parts come from the browser's own User-Agent and may be missing. */
export function deviceName(
  d: Pick<Device, 'browser' | 'os' | 'name'>,
  translate?: DeviceTranslate,
) {
  if (d.name) return d.name
  if (d.browser && d.os)
    return label(translate, 'browserOnOs', `${d.browser} on ${d.os}`, {
      browser: d.browser,
      os: d.os,
    })
  return d.browser || d.os || label(translate, 'unknownBrowser', 'Unknown browser')
}

export const mobileDevice = (d: Pick<Device, 'os'>) =>
  d.os === 'iOS' || d.os === 'iPadOS' || d.os === 'Android'

/** "just now", "5 minutes ago", "2 weeks ago"; exact times go in a title attribute. */
export function relativeTime(
  iso: string,
  now = Date.now(),
  locale = 'en',
  translate?: DeviceTranslate,
) {
  const seconds = Math.round((now - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return label(translate, 'justNow', 'just now')
  const format = new Intl.RelativeTimeFormat(locale, { numeric: 'always' })
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ['minute', 60],
    ['hour', 3600],
    ['day', 86_400],
    ['week', 7 * 86_400],
    ['month', 30 * 86_400],
    ['year', 365 * 86_400],
  ]
  let unit = steps[0]!
  for (const step of steps) if (seconds >= step[1]) unit = step
  return format.format(-Math.floor(seconds / unit[1]), unit[0])
}

export const exactTime = (iso: string, locale?: string, timeFormat?: '12h' | '24h') =>
  new Date(iso).toLocaleString(locale, {
    dateStyle: 'long',
    timeStyle: 'short',
    ...(timeFormat ? { hour12: timeFormat === '12h' } : {}),
  })

/** How recently the device showed signs of life: a live connection, or its last request. */
export function presence(
  d: Pick<Device, 'status' | 'active' | 'lastSeenAt'>,
  now = Date.now(),
  locale = 'en',
  translate?: DeviceTranslate,
) {
  if (d.active) return label(translate, 'activeNow', 'Active now')
  if (d.status === 'connected') return label(translate, 'connectedNow', 'Connected now')
  if (!d.lastSeenAt) return label(translate, 'lastSeenUnknown', 'Last seen: unknown')
  const time = relativeTime(d.lastSeenAt, now, locale, translate)
  return label(translate, 'lastSeen', `Last seen ${time}`, { time })
}

/** The desired delivery mode is separate from whether push is registered. */
export function notificationText(
  d: Pick<Device, 'push' | 'notifications'>,
  translate?: DeviceTranslate,
) {
  if (!d.notifications)
    return d.push === 'registered'
      ? label(translate, 'pushRegistered', 'Push registered')
      : label(translate, 'noPush', 'No push')
  if (d.notifications.delivery === 'push')
    return d.push === 'registered'
      ? label(translate, 'notificationsPush', 'Notifications: push')
      : label(translate, 'notificationsPushNotRegistered', 'Notifications: push (not registered)')
  if (d.notifications.delivery === 'open')
    return label(translate, 'notificationsWhileOpen', 'Notifications: while open')
  return label(translate, 'notificationsOff', 'Notifications: off')
}
