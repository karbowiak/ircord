// Notification rules. The backend owns them (it sends pushes while no client is open); the
// browser evaluates the same rules for in-page alerts, unread badges and highlighting.
import type { Conversation, Message } from './types/chat'
import { mentionsNick } from './utils/identity'
import { stripFormatting } from './utils/irc'

export type NotifyLevel = 'all' | 'mentions' | 'none'
export interface NotificationSettings {
  channelDefault: NotifyLevel
  directDefault: NotifyLevel
  /** Per-conversation overrides. */
  levels: Record<string, NotifyLevel>
  /** Extra words that count as a mention, besides your nick. */
  highlightWords: string[]
  /** Nicks or nick!user@host masks (with * and ?) whose messages are hidden and never notify. */
  ignore: string[]
  /**
   * No notifications (push, in-app or sound) between `start` and `end` in the user's time zone.
   * Times are “HH:MM”; a range past midnight (22:00–07:00) wraps.
   */
  quietHours: { enabled: boolean; start: string; end: string; timeZone: string }
}

export const defaultNotificationSettings: NotificationSettings = {
  channelDefault: 'mentions',
  directDefault: 'all',
  levels: {},
  highlightWords: [],
  ignore: [],
  quietHours: { enabled: false, start: '22:00', end: '07:00', timeZone: 'UTC' },
}
export const notificationLimits = { highlightWords: 50, ignore: 200, levels: 500 } as const
const levels: NotifyLevel[] = ['all', 'mentions', 'none']
const isLevel = (value: unknown): value is NotifyLevel => levels.includes(value as NotifyLevel)
const words = (value: unknown, limit: number, max: number) =>
  Array.isArray(value)
    ? [
        ...new Set(
          value
            .filter((w): w is string => typeof w === 'string')
            .map((w) => w.trim())
            .filter((w) => w.length > 0 && w.length <= max && !/[\r\n\0]/.test(w)),
        ),
      ].slice(0, limit)
    : []

export function sanitizeNotificationSettings(input: unknown): NotificationSettings {
  const v = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const entries = v.levels && typeof v.levels === 'object' ? Object.entries(v.levels) : []
  return {
    channelDefault: isLevel(v.channelDefault) ? v.channelDefault : 'mentions',
    directDefault: isLevel(v.directDefault) ? v.directDefault : 'all',
    levels: Object.fromEntries(
      entries
        .filter(([id, level]) => id.length > 0 && id.length <= 120 && isLevel(level))
        .slice(0, notificationLimits.levels),
    ) as Record<string, NotifyLevel>,
    highlightWords: words(v.highlightWords, notificationLimits.highlightWords, 64),
    ignore: words(v.ignore, notificationLimits.ignore, 120),
    quietHours: sanitizeQuietHours(v.quietHours),
  }
}

const time = /^([01]\d|2[0-3]):[0-5]\d$/
function sanitizeQuietHours(input: unknown): NotificationSettings['quietHours'] {
  const v = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const zone = typeof v.timeZone === 'string' && validTimeZone(v.timeZone) ? v.timeZone : 'UTC'
  return {
    enabled: v.enabled === true,
    start: typeof v.start === 'string' && time.test(v.start) ? v.start : '22:00',
    end: typeof v.end === 'string' && time.test(v.end) ? v.end : '07:00',
    timeZone: zone,
  }
}
function validTimeZone(zone: string) {
  try {
    new Intl.DateTimeFormat('en', { timeZone: zone })
    return true
  } catch {
    return false
  }
}

/** Whether `now` falls inside the quiet hours, in the user's own time zone. */
export function isQuiet(settings: NotificationSettings, now = new Date()) {
  const quiet = settings.quietHours
  if (!quiet.enabled || quiet.start === quiet.end) return false
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: quiet.timeZone,
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).format(now)
  const minutes = (hhmm: string) => Number(hhmm.slice(0, 2)) * 60 + Number(hhmm.slice(3, 5))
  const current = minutes(parts)
  const start = minutes(quiet.start)
  const end = minutes(quiet.end)
  return start < end ? current >= start && current < end : current >= start || current < end
}

/** A conversation as far as notification levels need it. */
export type LevelTarget = Pick<Conversation, 'id' | 'kind'> & Partial<Pick<Conversation, 'channelSettings'>>

/**
 * The effective level. A channel's level is channel-wide (set by its operators); its Default
 * uses your own channel default. Per-conversation levels you save apply to direct messages
 * only, so an old personal value for a channel never overrides the channel's setting.
 */
export function levelFor(settings: NotificationSettings, c: LevelTarget): NotifyLevel {
  if (c.kind === 'direct') return settings.levels[c.id] ?? settings.directDefault
  const shared = c.channelSettings?.notifications
  return shared && shared !== 'default' ? shared : settings.channelDefault
}

const escape = (value: string) => value.replace(/[.+^${}()|[\]\\]/g, '\\$&')
/** IRC-style mask: `nick`, or `nick!user@host` with `*` and `?` wildcards; case-insensitive. */
export function maskMatches(mask: string, nick: string, user = '*', host = '*') {
  const full = mask.includes('!') || mask.includes('@') ? mask : `${mask}!*@*`
  const pattern = new RegExp(`^${escape(full).replace(/\*/g, '.*').replace(/\?/g, '.')}$`, 'i')
  return pattern.test(`${nick}!${user}@${host}`)
}
export const isIgnored = (
  settings: NotificationSettings,
  nick: string | undefined,
  user = '*',
  host = '*',
) => !!nick && settings.ignore.some((mask) => maskMatches(mask, nick, user, host))

/** Mentions your nick or one of your highlight words (whole words, case-insensitive). */
export function isHighlight(
  settings: NotificationSettings,
  text: string,
  nick: string | undefined,
) {
  const plain = stripFormatting(text)
  return (
    mentionsNick(plain, nick) ||
    settings.highlightWords.some((word) =>
      new RegExp(`(^|[^\\w-])${escape(word)}(?![\\w-])`, 'i').test(plain),
    )
  )
}

/**
 * Whether an incoming message should alert the user: null, `'mention'` (a highlight or a
 * direct message), or `'message'` (the conversation is set to all messages).
 */
export function notificationFor(
  settings: NotificationSettings,
  c: LevelTarget,
  message: Pick<Message, 'text' | 'kind' | 'senderId'>,
  ownNick: string | undefined,
  senderNick: string | undefined,
  now = new Date(),
  sender?: { username?: string; host?: string },
): 'mention' | 'message' | null {
  if (message.senderId === 'you' || message.kind === 'event') return null
  if (isQuiet(settings, now)) return null
  if (isIgnored(settings, senderNick, sender?.username, sender?.host)) return null
  const level = levelFor(settings, c)
  if (level === 'none') return null
  const mention = c.kind === 'direct' || isHighlight(settings, message.text, ownNick)
  if (mention) return 'mention'
  return level === 'all' ? 'message' : null
}
