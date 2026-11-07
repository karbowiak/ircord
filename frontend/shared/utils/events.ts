import type { Message } from '../types/chat'

/** Membership churn that may be folded into one summary row; everything else stands alone. */
const chatterTypes = new Set(['join', 'part', 'quit', 'nick'])
export const isChatter = (message: Message) =>
  message.kind === 'event' && chatterTypes.has(message.event?.type ?? '')

/** A quit or part reason worth showing: empty, bare “Quit:” and “Client Quit” say nothing. */
export function eventReason(text?: string) {
  const reason = (text ?? '').trim().replace(/^Quit:\s*/i, '')
  return /^client quit$/i.test(reason) ? '' : reason
}

const ranks: Record<string, string> = {
  q: 'a founder',
  a: 'an admin',
  o: 'an operator',
  h: 'a half-operator',
}
const flags: Record<string, [string, string]> = {
  i: ['made the channel invite-only', 'made the channel open to join'],
  m: ['moderated the channel', 'unmoderated the channel'],
  t: ['limited topic changes to operators', 'let anyone change the topic'],
  n: ['blocked messages from outside the channel', 'allowed messages from outside the channel'],
  s: ['made the channel secret', 'made the channel visible'],
  p: ['made the channel private', 'made the channel public'],
  r: ['limited the channel to registered users', 'opened the channel to unregistered users'],
}
/** Modes that take an argument when set, and when unset (`l` takes none to unset). */
const withArgument = { set: 'qaohvbeIklfj', unset: 'qaohvbeIk' }

/**
 * A channel mode change as plain sentences: “+o-v sage noah” → “made sage an operator and took
 * voice from noah”. Modes without a known meaning keep their raw form.
 */
export function modeDescription(text: string) {
  const [changes = '', ...args] = text.trim().split(/\s+/)
  const parts: string[] = []
  const raw: string[] = []
  let sign = '+'
  for (const mode of changes) {
    if (mode === '+' || mode === '-') {
      sign = mode
      continue
    }
    const set = sign === '+'
    const arg = (set ? withArgument.set : withArgument.unset).includes(mode) ? args.shift() : ''
    if (ranks[mode] && arg)
      parts.push(
        set
          ? `made ${arg} ${ranks[mode]}`
          : `removed ${ranks[mode]!.replace(/^an? /, '')} from ${arg}`,
      )
    else if (mode === 'v' && arg)
      parts.push(set ? `gave voice to ${arg}` : `took voice from ${arg}`)
    else if (mode === 'b' && arg) parts.push(`${set ? 'banned' : 'unbanned'} ${arg}`)
    else if (mode === 'e' && arg)
      parts.push(`${set ? 'added' : 'removed'} a ban exception for ${arg}`)
    else if (mode === 'I' && arg)
      parts.push(`${set ? 'added' : 'removed'} an invite exception for ${arg}`)
    else if (mode === 'k') parts.push(set ? 'set a channel key' : 'removed the channel key')
    else if (mode === 'l')
      parts.push(set && arg ? `limited the channel to ${arg} users` : 'removed the user limit')
    else if (flags[mode]) parts.push(flags[mode]![set ? 0 : 1])
    else raw.push(`${sign}${mode}${arg ? ` ${arg}` : ''}`)
  }
  if (raw.length) parts.push(`set mode ${raw.join(' ')}`)
  return parts.length ? list(parts) : `set mode ${text}`
}

/** “a”, “a and b”, “a, b and c”. */
export function list(items: string[]) {
  return items.length < 2
    ? (items[0] ?? '')
    : `${items.slice(0, -1).join(', ')} and ${items.at(-1)}`
}

/** Names when there are a few, a count when there are many: “ada, lin and noah” / “12 people”. */
const people = (names: string[]) => (names.length <= 3 ? list(names) : `${names.length} people`)

/**
 * A run of joins, parts, quits and nick changes by outcome rather than by line: someone who
 * left and came back “reconnected”, someone who came and went “joined and left”.
 */
export function chatterSummary(events: Message[], nickOf: (memberId: string) => string) {
  const outcomes = new Map<string, { first: string; last: string; nick?: string }>()
  for (const m of events) {
    const e = m.event!
    const seen = outcomes.get(e.actorId)
    if (e.type === 'nick') {
      if (seen) seen.nick = e.text
      else outcomes.set(e.actorId, { first: 'nick', last: 'nick', nick: e.text })
    } else if (seen) seen.last = e.type === 'join' ? 'join' : 'leave'
    else {
      const kind = e.type === 'join' ? 'join' : 'leave'
      outcomes.set(e.actorId, { first: kind, last: kind })
    }
  }
  const groups = {
    reconnected: [] as string[],
    joined: [] as string[],
    'joined and left': [] as string[],
    left: [] as string[],
  }
  const renames: string[] = []
  for (const [id, o] of outcomes) {
    const name = nickOf(id)
    if (o.nick) renames.push(`${name} → ${o.nick}`)
    if (o.first === 'nick') continue
    if (o.first === 'leave') groups[o.last === 'join' ? 'reconnected' : 'left'].push(name)
    else groups[o.last === 'join' ? 'joined' : 'joined and left'].push(name)
  }
  const parts = Object.entries(groups)
    .filter(([, names]) => names.length)
    .map(([what, names]) => `${people(names)} ${what}`)
  if (renames.length)
    parts.push(renames.length <= 3 ? renames.join(', ') : `${renames.length} nick changes`)
  return parts.join(' · ')
}
