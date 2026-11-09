import type { Message } from '../types/chat'
import { eventReason, modeDescription, type EventTranslator } from './events'
import { stripFormatting } from './irc'

const pad = (n: number) => String(n).padStart(2, '0')
const eventMessages: Record<string, string> = {
  'collectionsUi.events.joined': 'joined',
  'collectionsUi.events.left': 'left',
  'collectionsUi.events.leftReason': 'left ({reason})',
  'collectionsUi.events.disconnected': 'disconnected',
  'collectionsUi.events.disconnectedReason': 'disconnected ({reason})',
  'collectionsUi.events.changedNick': 'changed nickname to {nick}',
  'collectionsUi.events.kicked': 'kicked {target}',
  'collectionsUi.events.kickedReason': 'kicked {target} ({reason})',
  'collectionsUi.events.invited': 'invited {target} to {channel}',
  'collectionsUi.events.changedTopic': 'changed the topic to “{topic}”',
  'collectionsUi.events.clearedTopic': 'cleared the topic',
  'collectionsUi.events.registered': 'created this channel, so it’s registered to your account',
  'collectionsUi.events.registrationFailed':
    'created this channel, but it couldn’t be registered to your account.',
  'collectionsUi.events.registrationFailedReason':
    'created this channel, but it couldn’t be registered to your account. {reason}',
}
const englishEventDescription: EventTranslator = (key, values) =>
  Object.entries(values ?? {}).reduce(
    (message, [name, value]) => message.replaceAll('{' + name + '}', String(value)),
    eventMessages[key] ?? key,
  )

/** Local time as “2026-11-07 10:13”. */
export function logTime(iso: string) {
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/**
 * What a membership or channel event did, after the actor’s nick: “left (bye)”. Without
 * `withReason`, part, quit and kick reasons are left for the caller to show separately.
 */
export function eventDescription(
  message: Message,
  nickOf: (memberId: string) => string,
  withReason = true,
  translate?: EventTranslator,
  locale = 'en',
) {
  const e = message.event
  if (!e) return stripFormatting(message.text)
  const t = translate ?? englishEventDescription
  const why = withReason ? eventReason(e.text) : ''
  switch (e.type) {
    case 'join':
      return t('collectionsUi.events.joined')
    case 'part':
      return t(why ? 'collectionsUi.events.leftReason' : 'collectionsUi.events.left', {
        reason: why,
      })
    case 'quit':
      return t(
        why ? 'collectionsUi.events.disconnectedReason' : 'collectionsUi.events.disconnected',
        { reason: why },
      )
    case 'nick':
      return t('collectionsUi.events.changedNick', { nick: e.text ?? '' })
    case 'kick':
      return t(why ? 'collectionsUi.events.kickedReason' : 'collectionsUi.events.kicked', {
        target: nickOf(e.targetId ?? ''),
        reason: why,
      })
    case 'invite':
      return t('collectionsUi.events.invited', {
        target: nickOf(e.targetId ?? ''),
        channel: e.text ?? '',
      })
    case 'topic':
      return e.text
        ? t('collectionsUi.events.changedTopic', { topic: e.text })
        : t('collectionsUi.events.clearedTopic')
    case 'mode':
      return modeDescription(e.text ?? '', translate, locale)
    case 'registered':
      return t('collectionsUi.events.registered')
    case 'registration-failed':
      return e.text
        ? t('collectionsUi.events.registrationFailedReason', { reason: e.text })
        : t('collectionsUi.events.registrationFailed')
  }
}

/** One message as an IRC log line: `[time] <nick> text`, `* nick action`, `-nick- notice`, `*** event`. */
export function logLine(
  m: Message,
  nickOf: (memberId: string) => string,
  timeOf: (iso: string) => string = logTime,
) {
  const at = `[${timeOf(m.createdAt)}]`
  if (m.kind === 'event')
    return `${at} *** ${m.event ? `${nickOf(m.event.actorId)} ${eventDescription(m, nickOf)}` : stripFormatting(m.text)}`
  const nick = nickOf(m.senderId)
  const text = stripFormatting(m.gif?.url ?? m.text).replace(/\r?\n/g, ' ↵ ')
  const extra = m.attachment && !text.includes(m.attachment.url) ? ` ${m.attachment.url}` : ''
  if (m.kind === 'action') return `${at} * ${nick} ${text}${extra}`
  if (m.kind === 'notice') return `${at} -${nick}- ${text}${extra}`
  return `${at} <${nick}> ${text}${extra}`
}

/**
 * A conversation as a plain-text IRC log, the way irssi or mIRC write them:
 * `[time] <nick> text`, `* nick action`, `-nick- notice`, `*** event`.
 */
export function formatLog(
  messages: Message[],
  nickOf: (memberId: string) => string,
  header?: string,
) {
  const lines = header ? [header, ''] : []
  for (const m of messages) lines.push(logLine(m, nickOf))
  return lines.join('\n') + '\n'
}

/**
 * Selected timeline messages as the clipboard gets them: a day line, then one log line each
 * with the short time shown on screen, and another day line wherever the selection crosses
 * midnight. `dayOf` should give an absolute date, since “Today” is wrong once pasted.
 */
export function selectionLog(
  messages: Message[],
  nickOf: (memberId: string) => string,
  timeOf: (iso: string) => string,
  dayOf: (iso: string) => string,
) {
  const lines: string[] = []
  let day = ''
  for (const m of messages) {
    const next = new Date(m.createdAt).toDateString()
    if (next !== day) lines.push(`--- ${dayOf(m.createdAt)} ---`)
    day = next
    lines.push(logLine(m, nickOf, timeOf))
  }
  return lines.join('\n')
}
