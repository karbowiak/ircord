/**
 * Profile presentation: avatar frame presets, a person's local time and language names. The
 * backend stores the frame as a preset id only; how each preset looks lives here.
 */

/** Avatar frames, in the order Settings offers them. `none` shows no frame. */
export const avatarFrames = ['none', 'ring', 'glow', 'gradient', 'gold', 'dashed'] as const
export type AvatarFrame = (typeof avatarFrames)[number]
export const isAvatarFrame = (value: string | undefined): value is AvatarFrame =>
  !!value && (avatarFrames as readonly string[]).includes(value)

/**
 * The time where they are, such as “14:32”, or undefined when the timezone is unknown to this
 * browser. `hour12` follows the reader's clock setting.
 */
export function localTime(
  timezone: string | undefined,
  now = new Date(),
  locale = 'en',
  hour12?: boolean,
) {
  if (!timezone) return undefined
  try {
    return new Intl.DateTimeFormat(locale, {
      hour: '2-digit',
      minute: '2-digit',
      hour12,
      timeZone: timezone,
    }).format(now)
  } catch {
    return undefined
  }
}

/** “Danish” for `da`; the tag itself when the browser can't name it. */
export function languageName(tag: string, locale = 'en') {
  try {
    return new Intl.DisplayNames([locale], { type: 'language' }).of(tag) ?? tag
  } catch {
    return tag
  }
}

/** A custom status still showing: set, and not past its deadline. */
export function statusShowing(
  status: { statusText?: string; statusEmoji?: string; statusExpiresAt?: string },
  now = Date.now(),
) {
  if (!status.statusText && !status.statusEmoji) return false
  const until = status.statusExpiresAt ? Date.parse(status.statusExpiresAt) : NaN
  return Number.isNaN(until) || until > now
}

/** When a status set now should clear, for the choices Settings offers. */
export function statusDeadline(choice: string, now = new Date()): string | null {
  const at = new Date(now)
  switch (choice) {
    case '30m':
      at.setMinutes(at.getMinutes() + 30)
      break
    case '1h':
      at.setHours(at.getHours() + 1)
      break
    case '4h':
      at.setHours(at.getHours() + 4)
      break
    case 'today':
      at.setHours(23, 59, 59, 0)
      break
    case 'week':
      at.setDate(at.getDate() + 7)
      break
    default:
      return null
  }
  return at.toISOString()
}
