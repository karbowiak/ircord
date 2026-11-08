// Device activity (#84, #136): the form factor a tab reports and how a routed alert reads.
import type { components } from '../generated/api'

export type FormFactor = 'desktop' | 'phone' | 'tablet'
export type NotificationAlert = components['schemas']['Alert']

interface DeviceHints {
  userAgent: string
  platform?: string
  maxTouchPoints?: number
  /** Whether the primary pointer is coarse (a finger). */
  coarse?: boolean
  /** The shorter screen side in CSS pixels. */
  shortSide?: number
}

/**
 * Phone, tablet or desktop, from the User-Agent and touch hints. iPadOS Safari reports a Mac
 * User-Agent, so a touch-capable “Mac” is a tablet; Android tablets leave “Mobile” out.
 */
export function formFactor(hints: DeviceHints = browserHints()): FormFactor {
  const ua = hints.userAgent
  if (/iPhone|iPod/.test(ua) || (/Android/.test(ua) && /Mobile/.test(ua))) return 'phone'
  if (/iPad|Tablet/.test(ua) || /Android/.test(ua)) return 'tablet'
  if (/Mac/.test(hints.platform ?? ua) && (hints.maxTouchPoints ?? 0) > 1) return 'tablet'
  if (hints.coarse && (hints.shortSide ?? Infinity) < 600) return 'phone'
  return 'desktop'
}

function browserHints(): DeviceHints {
  if (typeof navigator === 'undefined') return { userAgent: '' }
  return {
    userAgent: navigator.userAgent,
    platform: navigator.platform,
    maxTouchPoints: navigator.maxTouchPoints,
    coarse: typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches,
    shortSide: typeof screen === 'undefined' ? undefined : Math.min(screen.width, screen.height),
  }
}

/** The title and body a page notification shows for a routed alert. */
export function alertText(alert: NotificationAlert): { title: string; body: string } {
  if (!alert.reminder) return { title: alert.title, body: alert.body }
  const where = alert.conversations > 1 ? ` in ${alert.conversations} conversations` : ''
  return { title: `${alert.unread} unread notifications${where}`, body: `${alert.title}: ${alert.body}` }
}
