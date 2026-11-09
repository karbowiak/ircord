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
 * The composer's Markdown for a message's IRC formatting, the inverse of markdownToIrc, so an
 * edit starts from what you typed. Colors have no Markdown form and are dropped.
 */
export function ircToMarkdown(text: string) {
  if (!controls.test(text)) return text
  return parseFormatting(text)
    .map((run) => {
      let out = run.mono ? `\`${run.text}\`` : run.text
      if (run.strike) out = `~~${out}~~`
      if (run.underline) out = `__${out}__`
      if (run.italic) out = `*${out}*`
      if (run.bold) out = `**${out}**`
      return out
    })
    .join('')
}

/** A fenced ``` code block, or the ordinary text between blocks. `raw` is the exact source. */
export type CodeSegment =
  { kind: 'text'; raw: string } | { kind: 'code'; raw: string; code: string; lang?: string }

/**
 * Splits out ``` fenced code blocks, as Discord and Markdown write them. A fence opens at the
 * start of a line, may name a language on its first line, and closes at the next ```. An
 * unterminated fence stays ordinary text. Fences travel on IRC exactly as typed, so other clients
 * read plain Markdown and every IRCord view (live, history, replay) renders from the same text.
 * Linear in the input: every scan moves forward.
 */
export function splitCodeBlocks(text: string): CodeSegment[] {
  if (!text.includes('```')) return [{ kind: 'text', raw: text }]
  const segments: CodeSegment[] = []
  let pending = 0
  let at = 0
  while ((at = text.indexOf('```', at)) !== -1) {
    if (at > 0 && text[at - 1] !== '\n') {
      at += 3
      continue
    }
    const close = text.indexOf('```', at + 3)
    if (close === -1) break
    let code = text.slice(at + 3, close)
    const hint = /^([\w+#.-]{1,32})?[ \t]*\n/.exec(code)
    const lang = hint?.[1]
    if (hint) code = code.slice(hint[0].length)
    if (code.endsWith('\n')) code = code.slice(0, -1)
    if (!code.trim()) {
      at += 3
      continue
    }
    if (at > pending) segments.push({ kind: 'text', raw: text.slice(pending, at) })
    segments.push({ kind: 'code', raw: text.slice(at, close + 3), code, ...(lang ? { lang } : {}) })
    pending = at = close + 3
  }
  if (pending < text.length) segments.push({ kind: 'text', raw: text.slice(pending) })
  return segments
}

/** Text outside code (fenced blocks and IRC monospace), e.g. for finding links to preview. */
export function textOutsideCode(text: string) {
  return splitCodeBlocks(text)
    .map((segment) =>
      segment.kind === 'code'
        ? ' '
        : parseFormatting(segment.raw)
            .map((run) => (run.mono ? ' ' : run.text))
            .join(''),
    )
    .join('')
}

/**
 * Turns the composer's Discord-style shortcuts into IRC codes so every IRC client sees the
 * formatting: **bold**, *italic* or _italic_, __underline__, ~~strike~~, `inline code` (IRC
 * monospace). ``` fenced blocks are sent untouched. Nothing inside code, URLs or words such as
 * snake_case is converted.
 */
export function markdownToIrc(text: string) {
  return splitCodeBlocks(text)
    .map((segment) => (segment.kind === 'code' ? segment.raw : inlineToIrc(segment.raw)))
    .join('')
}
function inlineToIrc(text: string) {
  // Inline code and URLs are set aside behind NUL placeholders (NUL is never valid in an IRC
  // message) while the emphasis rules run, so emphasis may still wrap inline code, and a URL
  // that is followed by a space: a URL match runs to the next space and takes a touching
  // closing marker (`**https://x.test**`) with it.
  const kept: string[] = []
  /* eslint-disable no-control-regex -- NUL marks protected spans; IRC formatting codes are control bytes */
  return text
    .replace(/\x00/g, '')
    .replace(/`[^`\n]+`|(?:https?|ircs?):\/\/\S+/g, (match) => {
      kept.push(match.startsWith('`') ? `\x11${match.slice(1, -1)}\x11` : match)
      return `\x00${kept.length - 1}\x00`
    })
    .replace(/\*\*([^*\n]+)\*\*/g, '\x02$1\x02')
    .replace(/__([^_\n]+)__/g, '\x1f$1\x1f')
    .replace(/~~([^~\n]+)~~/g, '\x1e$1\x1e')
    .replace(/(^|[^\w*])\*([^*\s][^*\n]*?)\*(?![\w*])/g, '$1\x1d$2\x1d')
    .replace(/(^|[^\w])_([^_\s][^_\n]*?)_(?!\w)/g, '$1\x1d$2\x1d')
    .replace(/\x00(\d+)\x00/g, (_, index: string) => kept[Number(index)]!)
  /* eslint-enable no-control-regex */
}

/**
 * The text a send puts on the wire: markdown shortcuts become IRC formatting unless the text is
 * `verbatim` (a GIF or upload URL, or text IRCord composed, like the nick in a `/slap`).
 */
export function wireText(text: string, verbatim = false) {
  return verbatim ? text.trim() : markdownToIrc(text.trim())
}
