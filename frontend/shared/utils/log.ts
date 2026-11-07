import type { Message } from '../types/chat'
import { stripFormatting } from './irc'

const pad = (n: number) => String(n).padStart(2, '0')
/** Local time as “2026-11-07 10:13”. */
export function logTime(iso: string) {
  const d = new Date(iso)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
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
  for (const m of messages) {
    const at = `[${logTime(m.createdAt)}]`
    const nick = nickOf(m.senderId)
    const text = stripFormatting(m.gif?.url ?? m.text).replace(/\r?\n/g, ' ↵ ')
    const extra = m.attachment && !text.includes(m.attachment.url) ? ` ${m.attachment.url}` : ''
    if (m.kind === 'event') lines.push(`${at} *** ${stripFormatting(m.text)}`)
    else if (m.kind === 'action') lines.push(`${at} * ${nick} ${text}${extra}`)
    else if (m.kind === 'notice') lines.push(`${at} -${nick}- ${text}${extra}`)
    else lines.push(`${at} <${nick}> ${text}${extra}`)
  }
  return lines.join('\n') + '\n'
}
