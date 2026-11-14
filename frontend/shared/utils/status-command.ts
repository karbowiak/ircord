/** The profile fields changed by /status; all other profile fields are left alone. */
export interface StatusUpdate {
  statusText: string
  statusEmoji: string
  statusExpiresAt: string
}

export type StatusCommand =
  { kind: 'edit' } | { kind: 'invalid' } | { kind: 'clear' | 'set'; input: StatusUpdate }

const maxDuration = 365 * 24 * 60 * 60 * 1000
const bytes = (value: string) => new TextEncoder().encode(value).length

/** Recognize one complete emoji grapheme, including flags, keycaps and ZWJ sequences. */
function isEmoji(value: string): boolean {
  const graphemes = [...new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(value)]
  return (
    graphemes.length === 1 &&
    (/\p{Extended_Pictographic}/u.test(value) ||
      /^\p{Regional_Indicator}{2}$/u.test(value) ||
      /^[0-9#*]\uFE0F?\u20E3$/u.test(value))
  )
}

/** Parse the arguments to /status. Deadlines use elapsed time, independent of local timezone. */
export function parseStatusCommand(argument: string, now = Date.now()): StatusCommand {
  const rest = argument.trim()
  if (!rest) return { kind: 'edit' }
  if (rest === 'clear')
    return { kind: 'clear', input: { statusText: '', statusEmoji: '', statusExpiresAt: '' } }
  if (/[\p{Cc}\p{Cs}]/u.test(rest)) return { kind: 'invalid' }

  const words = rest.split(/\s+/u)
  const message: string[] = []
  let emoji: string | undefined
  let duration: string | undefined
  for (let i = 0; i < words.length; i++) {
    const word = words[i]!
    if (word.startsWith('--')) {
      if (word !== '--emoji' && word !== '--for') return { kind: 'invalid' }
      const value = words[++i]
      if (!value || value.startsWith('--')) return { kind: 'invalid' }
      if (word === '--emoji') {
        if (emoji !== undefined) return { kind: 'invalid' }
        emoji = value
      } else {
        if (duration !== undefined) return { kind: 'invalid' }
        duration = value
      }
    } else {
      message.push(word)
    }
  }
  if (message[0] && isEmoji(message[0])) {
    if (emoji !== undefined) return { kind: 'invalid' }
    emoji = message.shift()
  }
  const statusText = message.join(' ')
  const statusEmoji = emoji ?? ''
  if (!statusText || bytes(statusText) > 200 || bytes(statusEmoji) > 64) return { kind: 'invalid' }

  let statusExpiresAt = ''
  if (duration !== undefined && duration !== 'never') {
    const match = /^(\d+)([mhd])$/.exec(duration)
    if (!match) return { kind: 'invalid' }
    const unit = { m: 60_000, h: 3_600_000, d: 86_400_000 }[match[2] as 'm' | 'h' | 'd']
    const elapsed = Number(match[1]) * unit
    if (!Number.isSafeInteger(elapsed) || elapsed <= 0 || elapsed > maxDuration)
      return { kind: 'invalid' }
    statusExpiresAt = new Date(now + elapsed).toISOString()
  }
  return { kind: 'set', input: { statusText, statusEmoji, statusExpiresAt } }
}

/** Execute only profile updates, with the network captured by the caller at submission time. */
export async function runStatusCommand(
  argument: string,
  networkId: string | undefined,
  deps: {
    edit: (networkId: string) => boolean
    update: (networkId: string, input: StatusUpdate) => Promise<unknown>
    notify: (key: 'usage' | 'network' | 'editor' | 'set' | 'cleared') => void
  },
): Promise<'handled' | 'kept'> {
  const command = parseStatusCommand(argument)
  if (command.kind === 'invalid') {
    deps.notify('usage')
    return 'kept'
  }
  if (!networkId) {
    deps.notify('network')
    return 'kept'
  }
  if (command.kind === 'edit') {
    if (deps.edit(networkId)) return 'handled'
    deps.notify('editor')
    return 'kept'
  }
  await deps.update(networkId, command.input)
  deps.notify(command.kind === 'clear' ? 'cleared' : 'set')
  return 'handled'
}
