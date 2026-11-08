import type { components } from '../generated/api'

export type Device = components['schemas']['Device']

/** "Firefox on Linux"; the parts come from the browser's own User-Agent and may be missing. */
export function deviceName(d: Pick<Device, 'browser' | 'os'>) {
  if (d.browser && d.os) return `${d.browser} on ${d.os}`
  return d.browser || d.os || 'Unknown browser'
}

export const mobileDevice = (d: Pick<Device, 'os'>) =>
  d.os === 'iOS' || d.os === 'iPadOS' || d.os === 'Android'

/** "just now", "5 minutes ago", "3 days ago"; exact times go in a title attribute. */
export function relativeTime(iso: string, now = Date.now()) {
  const seconds = Math.round((now - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return 'just now'
  const format = new Intl.RelativeTimeFormat('en', { numeric: 'always' })
  const steps: [Intl.RelativeTimeFormatUnit, number][] = [
    ['minute', 60],
    ['hour', 3600],
    ['day', 86_400],
    ['month', 30 * 86_400],
    ['year', 365 * 86_400],
  ]
  let unit = steps[0]!
  for (const step of steps) if (seconds >= step[1]) unit = step
  return format.format(-Math.floor(seconds / unit[1]), unit[0])
}

export const exactTime = (iso: string) =>
  new Date(iso).toLocaleString(undefined, { dateStyle: 'long', timeStyle: 'short' })

/** How recently the device showed signs of life: a live connection, or its last request. */
export function presence(d: Pick<Device, 'status' | 'current' | 'lastSeenAt'>, now = Date.now()) {
  if (d.current) return 'Active now'
  if (d.status === 'connected') return 'Connected now'
  if (!d.lastSeenAt) return 'Last seen: unknown'
  return `Last seen ${relativeTime(d.lastSeenAt, now)}`
}

/** This device knows its own notification mode; others only say whether push is registered. */
export function notificationText(d: Pick<Device, 'current' | 'push'>, localMode?: string) {
  if (d.current && localMode === 'push')
    return d.push === 'registered' ? 'Notifications: push' : 'Notifications: push (not registered)'
  if (d.current && localMode === 'open') return 'Notifications: while open'
  if (d.current) return 'Notifications: off'
  return d.push === 'registered' ? 'Push registered' : 'No push'
}
