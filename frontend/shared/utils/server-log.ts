import type { ServerLine } from '../types/chat'

/** Routine traffic that is folded into one row when it arrives in a run. */
export type ServerLineFamily = 'markread' | 'presence' | 'away' | 'ping' | 'batch'

export type ServerLogItem =
  | { type: 'line'; line: ServerLine }
  | { type: 'group'; id: string; family: ServerLineFamily; lines: ServerLine[] }

/** A run this long or longer is folded; shorter runs stay as lines. */
export const minGroupSize = 3

const presence = /joined the channel|\bquit\b|left the channel|\bparted\b|changed (?:their )?nick/i

export function serverLineFamily(line: ServerLine): ServerLineFamily | null {
  const command = line.command.toUpperCase()
  if (command === 'MARKREAD') return 'markread'
  if (command === 'AWAY' || command === '305' || command === '306') return 'away'
  if (command === 'PING' || command === 'PONG') return 'ping'
  if (command === 'BATCH' || command === 'CHATHISTORY') return 'batch'
  if (['JOIN', 'PART', 'QUIT', 'NICK'].includes(command)) return 'presence'
  if (line.kind === 'notice' && /^histserv\b/i.test(line.source ?? '') && presence.test(line.text))
    return 'presence'
  return null
}

/** The server repeating back a command IRCord just sent: same command, same text. */
const isEcho = (previous: ServerLine | undefined, line: ServerLine) =>
  previous?.kind === 'outgoing' &&
  line.kind !== 'outgoing' &&
  previous.command.toUpperCase() === line.command.toUpperCase() &&
  previous.text === line.text

/**
 * The log as it reads best: echoes of sent commands dropped, and runs of routine lines of one
 * family folded into a group that can be opened. Everything else keeps its own row, in order.
 */
export function groupServerLog(lines: readonly ServerLine[]): ServerLogItem[] {
  const kept = lines.filter((line, i) => !isEcho(lines[i - 1], line))
  const items: ServerLogItem[] = []
  let run: ServerLine[] = []
  let family: ServerLineFamily | null = null
  const flush = () => {
    if (family && run.length >= minGroupSize)
      items.push({ type: 'group', id: `group:${run[0]!.id}`, family, lines: run })
    else items.push(...run.map((line) => ({ type: 'line' as const, line })))
    run = []
    family = null
  }
  for (const line of kept) {
    const next = serverLineFamily(line)
    if (next && next === family) {
      run.push(line)
      continue
    }
    flush()
    if (next) {
      family = next
      run = [line]
    } else items.push({ type: 'line', line })
  }
  flush()
  return items
}
