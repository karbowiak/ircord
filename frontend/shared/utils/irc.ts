// IRC protocol helpers shared by the browser and the Nitro mock: channel privileges from
// ISUPPORT, mIRC formatting codes, and raw line validation.
import type { Conversation, Network } from '../types/chat'

/** Keep the wire target distinct from the display name, including non-# channels. */
export function channelTarget(conversation: Pick<Conversation, 'name' | 'target'>) {
  return conversation.target ?? `#${conversation.name}`
}

// ---- Privileges ---------------------------------------------------------------------------

/** PREFIX=(qaohv)~&@%+ → modes "qaohv", prefixes "~&@%+" (highest first). */
export function parsePrefix(isupport: Record<string, string> | undefined) {
  const match = /^\(([a-zA-Z]+)\)(\S+)$/.exec(isupport?.PREFIX ?? '')
  return match && match[1]!.length === match[2]!.length
    ? { modes: match[1]!, prefixes: match[2]! }
    : { modes: 'qaohv', prefixes: '~&@%+' }
}

/** Position of a prefix in the network's PREFIX order (0 = highest), or Infinity. */
export function prefixRank(prefix: string, network: Pick<Network, 'isupport'> | undefined) {
  const { prefixes } = parsePrefix(network?.isupport)
  const ranks = [...prefix].map((char) => prefixes.indexOf(char)).filter((i) => i >= 0)
  return ranks.length ? Math.min(...ranks) : Infinity
}

/** The highest prefix character a member has in a channel, or ''. */
export function topPrefix(
  memberId: string,
  conversation: Pick<Conversation, 'memberModes'> | undefined,
  network: Pick<Network, 'isupport'> | undefined,
) {
  const own = conversation?.memberModes?.[memberId] ?? ''
  const { prefixes } = parsePrefix(network?.isupport)
  return [...prefixes].find((char) => own.includes(char)) ?? ''
}

/**
 * What a member may do in a channel. Halfop (%) and above can kick, ban and voice; op (@) and
 * above can also change ops. Topic needs halfop only when the channel is +t.
 */
export function channelRights(
  memberId: string,
  conversation: Pick<Conversation, 'memberModes' | 'modes' | 'kind' | 'joined'> | undefined,
  network: (Pick<Network, 'isupport'> & Partial<Pick<Network, 'status'>>) | undefined,
) {
  const { modes } = parsePrefix(network?.isupport)
  const rank = prefixRank(conversation?.memberModes?.[memberId] ?? '', network)
  const at = (char: string) => {
    const index = modes.indexOf(char)
    return index >= 0 && rank <= index
  }
  const halfop = at('h') || at('o')
  const op = at('o')
  const channel =
    conversation?.kind === 'channel' &&
    conversation.joined !== false &&
    (!network?.status || network.status === 'connected')
  return {
    kick: channel && halfop,
    ban: channel && halfop,
    voice: channel && halfop,
    op: channel && op,
    invite: channel && (!conversation?.modes?.includes('i') || halfop),
    topic: channel && (!conversation?.modes?.includes('t') || halfop),
  }
}
export type ChannelRights = ReturnType<typeof channelRights>

// ---- Raw lines ----------------------------------------------------------------------------

/** One IRC line from the client: no line breaks or NUL, at most 510 bytes. */
export function validRawLine(line: unknown): line is string {
  return (
    typeof line === 'string' &&
    line.trim().length > 0 &&
    !/[\r\n\0]/.test(line) &&
    new TextEncoder().encode(line).length <= 510
  )
}

// ---- Formatting codes ---------------------------------------------------------------------

/** mIRC colours 0–98 (ircdocs.horse/specs/formatting). */
export const ircColors = [
  'ffffff',
  '000000',
  '00007f',
  '009300',
  'ff0000',
  '7f0000',
  '9c009c',
  'fc7f00',
  'ffff00',
  '00fc00',
  '009393',
  '00ffff',
  '0000fc',
  'ff00ff',
  '7f7f7f',
  'd2d2d2',
  '470000',
  '472100',
  '474700',
  '324700',
  '004700',
  '00472c',
  '004747',
  '002747',
  '000047',
  '2e0047',
  '470047',
  '47002a',
  '740000',
  '743a00',
  '747400',
  '517400',
  '007400',
  '007449',
  '007474',
  '004074',
  '000074',
  '4b0074',
  '740074',
  '740045',
  'b50000',
  'b56300',
  'b5b500',
  '7db500',
  '00b500',
  '00b571',
  '00b5b5',
  '0063b5',
  '0000b5',
  '7500b5',
  'b500b5',
  'b5006b',
  'ff0000',
  'ff8c00',
  'ffff00',
  'b2ff00',
  '00ff00',
  '00ffa0',
  '00ffff',
  '008cff',
  '0000ff',
  'a500ff',
  'ff00ff',
  'ff0098',
  'ff5959',
  'ffb459',
  'ffff71',
  'cfff60',
  '6fff6f',
  '65ffc9',
  '6dffff',
  '59b4ff',
  '5959ff',
  'c459ff',
  'ff66ff',
  'ff59bc',
  'ff9c9c',
  'ffd39c',
  'ffff9c',
  'e2ff9c',
  '9cff9c',
  '9cffdb',
  '9cffff',
  '9cd3ff',
  '9c9cff',
  'dc9cff',
  'ff9cff',
  'ff94d3',
  '000000',
  '131313',
  '282828',
  '363636',
  '4d4d4d',
  '656565',
  '818181',
  '9f9f9f',
  'bcbcbc',
  'e2e2e2',
  'ffffff',
].map((hex) => `#${hex}`)

export interface FormattedRun {
  text: string
  bold?: boolean
  italic?: boolean
  underline?: boolean
  strike?: boolean
  mono?: boolean
  fg?: string
  bg?: string
}

// eslint-disable-next-line no-control-regex
const controls = /[\x02\x03\x04\x0f\x11\x16\x1d\x1e\x1f]/

/** Splits text into runs with their formatting. Plain text returns a single run. */
export function parseFormatting(text: string): FormattedRun[] {
  if (!controls.test(text)) return [{ text }]
  const runs: FormattedRun[] = []
  let style: Omit<FormattedRun, 'text'> = {}
  let reverse = false
  let buffer = ''
  const flush = () => {
    if (!buffer) return
    const run: FormattedRun = { text: buffer, ...style }
    if (reverse) [run.fg, run.bg] = [style.bg ?? 'var(--main)', style.fg ?? 'var(--body)']
    runs.push(run)
    buffer = ''
  }
  for (let i = 0; i < text.length; i++) {
    const char = text[i]!
    const toggle = (key: 'bold' | 'italic' | 'underline' | 'strike' | 'mono') => {
      flush()
      style = { ...style, [key]: !style[key] }
    }
    if (char === '\x02') toggle('bold')
    else if (char === '\x1d') toggle('italic')
    else if (char === '\x1f') toggle('underline')
    else if (char === '\x1e') toggle('strike')
    else if (char === '\x11') toggle('mono')
    else if (char === '\x16') {
      flush()
      reverse = !reverse
    } else if (char === '\x0f') {
      flush()
      style = {}
      reverse = false
    } else if (char === '\x03') {
      flush()
      const match = /^(\d{1,2})(?:,(\d{1,2}))?/.exec(text.slice(i + 1))
      if (!match) style = { ...style, fg: undefined, bg: undefined }
      else {
        i += match[0].length
        const fg = ircColors[Number(match[1])]
        const bg = match[2] !== undefined ? ircColors[Number(match[2])] : style.bg
        style = { ...style, fg, bg }
      }
    } else if (char === '\x04') {
      flush()
      const match = /^([\da-fA-F]{6})(?:,([\da-fA-F]{6}))?/.exec(text.slice(i + 1))
      if (!match) style = { ...style, fg: undefined, bg: undefined }
      else {
        i += match[0].length
        style = { ...style, fg: `#${match[1]}`, bg: match[2] ? `#${match[2]}` : style.bg }
      }
    } else buffer += char
  }
  flush()
  return runs
}

/** Text without formatting codes, for excerpts, search and notifications. */
export function stripFormatting(text: string) {
  return parseFormatting(text)
    .map((run) => run.text)
    .join('')
}

/**
 * Turns the composer's markdown-like shortcuts into IRC codes so every IRC client sees the
 * formatting: **bold**, *italic* or _italic_, __underline__, ~~strike~~, `monospace`. URLs and
 * words such as snake_case are left alone.
 */
export function markdownToIrc(text: string) {
  return text
    .split(/(https?:\/\/\S+)/g)
    .map((part, index) =>
      index % 2
        ? part
        : part
            .replace(/`([^`\n]+)`/g, '\x11$1\x11')
            .replace(/\*\*([^*\n]+)\*\*/g, '\x02$1\x02')
            .replace(/__([^_\n]+)__/g, '\x1f$1\x1f')
            .replace(/~~([^~\n]+)~~/g, '\x1e$1\x1e')
            .replace(/(^|[^\w*])\*([^*\s][^*\n]*?)\*(?![\w*])/g, '$1\x1d$2\x1d')
            .replace(/(^|[^\w])_([^_\s][^_\n]*?)_(?!\w)/g, '$1\x1d$2\x1d'),
    )
    .join('')
}
