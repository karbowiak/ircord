import type { Message } from '../types/chat'
import { eventReason, modeDescription, type EventTranslator } from './events'
import { stripFormatting } from './irc'

const pad = (n: number) => String(n).padStart(2, '0')
const eventMessages: Record<string, string> = {
  'collectionsUi.events.joined': '{actor} joined',
  'collectionsUi.events.left': '{actor} left',
  'collectionsUi.events.leftReason': '{actor} left ({reason})',
  'collectionsUi.events.disconnected': '{actor} disconnected',
  'collectionsUi.events.disconnectedReason': '{actor} disconnected ({reason})',
  'collectionsUi.events.changedNick': '{actor} changed nickname to {nick}',
  'collectionsUi.events.kicked': '{actor} kicked {target}',
  'collectionsUi.events.kickedReason': '{actor} kicked {target} ({reason})',
  'collectionsUi.events.invited': '{actor} invited {target} to {channel}',
  'collectionsUi.events.changedTopic': '{actor} changed the topic to “{topic}”',
  'collectionsUi.events.clearedTopic': '{actor} cleared the topic',
  'collectionsUi.events.mode': '{actor} changed channel modes: {changes}',
  'collectionsUi.events.registered':
    '{actor} created this channel, so it’s registered to your account',
  'collectionsUi.events.registrationFailed':
    '{actor} created this channel, but it couldn’t be registered to your account.',
  'collectionsUi.events.registrationFailedReason':
    '{actor} created this channel, but it couldn’t be registered to your account. {reason}',
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
 * Select the same semantic sentence for rich timeline rendering and plain-text logs.
 * Without `withReason`, part, quit and kick reasons are left for the caller to show separately.
 */
export function eventPresentation(
  message: Message,
  nickOf: (memberId: string) => string,
  withReason = true,
  translate?: EventTranslator,
  locale = 'en',
) {
  const e = message.event
  if (!e) return null
  const why = withReason ? eventReason(e.text) : ''
  const actor = nickOf(e.actorId)
  const sentence = (
    name: string,
    values: Record<string, string> = {},
  ): {
    key: string
    values: Record<string, string>
  } => ({
    key: `collectionsUi.events.${name}`,
    values: { actor, ...values },
  })
  switch (e.type) {
    case 'join':
      return sentence('joined')
    case 'part':
      return sentence(why ? 'leftReason' : 'left', {
        reason: why,
      })
    case 'quit':
      return sentence(why ? 'disconnectedReason' : 'disconnected', { reason: why })
    case 'nick':
      return sentence('changedNick', { nick: e.text ?? '' })
    case 'kick':
      return sentence(why ? 'kickedReason' : 'kicked', {
        target: nickOf(e.targetId ?? ''),
        reason: why,
      })
    case 'invite':
      return sentence('invited', {
        target: nickOf(e.targetId ?? ''),
        channel: e.text ?? '',
      })
    case 'topic':
      return e.text ? sentence('changedTopic', { topic: e.text }) : sentence('clearedTopic')
    case 'mode':
      return sentence('mode', { changes: modeDescription(e.text ?? '', translate, locale) })
    case 'registered':
      return sentence('registered')
    case 'registration-failed':
      return e.text
        ? sentence('registrationFailedReason', { reason: e.text })
        : sentence('registrationFailed')
  }
}

export function eventDescription(
  message: Message,
  nickOf: (memberId: string) => string,
  withReason = true,
  translate?: EventTranslator,
  locale = 'en',
) {
  // Keep the existing English log wording for callers that do not provide a translator.
  if (!translate && message.event?.type === 'mode')
    return `${nickOf(message.event.actorId)} ${modeDescription(message.event.text ?? '', undefined, locale)}`
  const presentation = eventPresentation(message, nickOf, withReason, translate, locale)
  return presentation
    ? (translate ?? englishEventDescription)(presentation.key, presentation.values)
    : stripFormatting(message.text)
}

/** One message as an IRC log line: `[time] <nick> text`, `* nick action`, `-nick- notice`, `*** event`. */
export function logLine(
  m: Message,
  nickOf: (memberId: string) => string,
  timeOf: (iso: string) => string = logTime,
  translate?: EventTranslator,
  locale = 'en',
) {
  const at = `[${timeOf(m.createdAt)}]`
  if (m.kind === 'event') return `${at} *** ${eventDescription(m, nickOf, true, translate, locale)}`
  const nick = nickOf(m.senderId)
  const text = stripFormatting(m.gif?.url ?? m.text).replace(/\r?\n/g, ' ↵ ')
  const extra = (m.attachments ?? (m.attachment ? [m.attachment] : []))
    .filter((attachment) => !text.includes(attachment.url))
    .map((attachment) => ` ${attachment.url}`)
    .join('')
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
  translate?: EventTranslator,
  locale = 'en',
) {
  const lines = header ? [header, ''] : []
  for (const m of messages) lines.push(logLine(m, nickOf, logTime, translate, locale))
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
  translate?: EventTranslator,
  locale = 'en',
) {
  const lines: string[] = []
  let day = ''
  for (const m of messages) {
    const next = new Date(m.createdAt).toDateString()
    if (next !== day) lines.push(`--- ${dayOf(m.createdAt)} ---`)
    day = next
    lines.push(logLine(m, nickOf, timeOf, translate, locale))
  }
  return lines.join('\n')
}
