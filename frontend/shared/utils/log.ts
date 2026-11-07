import type { Message } from '../types/chat'
import { stripFormatting } from './irc'

const pad = (n: number) => String(n).padStart(2, '0')
/** Local time as “2026-11-07 10:13”. */
export function logTime(iso: string) {
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** What a membership or channel event did, after the actor’s nick: “left (bye)”. */
export function eventDescription(message: Message, nickOf: (memberId: string) => string) {
  const e = message.event
  if (!e) return stripFormatting(message.text)
  const reason = e.text ? ` (${e.text})` : ''
  switch (e.type) {
    case 'join':
      return 'joined'
    case 'part':
      return `left${reason}`
    case 'quit':
      return `disconnected${reason}`
    case 'nick':
      return `changed nickname to ${e.text ?? ''}`
    case 'kick':
      return `kicked ${nickOf(e.targetId ?? '')}${reason}`
    case 'invite':
      return `invited ${nickOf(e.targetId ?? '')} to ${e.text ?? ''}`
    case 'topic':
      return `changed the topic: ${e.text ?? ''}`
    case 'mode': {
      const ban = /^([+-])b\s+(.+)$/.exec(e.text ?? '')
      return ban
        ? `${ban[1] === '+' ? 'banned' : 'unbanned'} ${ban[2]}`
        : `set mode ${e.text ?? ''}`
    }
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
