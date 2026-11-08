// The blocks a message body renders as: fenced code, quotes, Markdown lists and the text between.
import { splitCodeBlocks, stripFormatting } from './irc'
import { splitLists, type ListBlock } from './lists'
import { splitQuotes } from './quotes'

export type MessageBlock =
  | ListBlock
  | { kind: 'code'; code: string; lang?: string }
  | { kind: 'quote'; blocks: MessageBlock[] }

/**
 * Splits a message body into blocks. Fenced code comes first, so nothing inside it is a quote or
 * a list; then `> ` quotes, whose contents split into code and lists again; then lists. A `>>> `
 * quote takes everything after it, code blocks included. The line breaks around a block belong to
 * the block, so the text next to it drops one newline on that side. With `continuesLine` the
 * first line follows other content (an action's nick), so it starts no quote or list.
 */
export function messageBlocks(
  text: string,
  { quotes = true, continuesLine = false } = {},
): MessageBlock[] {
  if (continuesLine) {
    const end = text.indexOf('\n')
    if (end < 0) return [{ kind: 'text', text }]
    const head = text.slice(0, end)
    const rest = messageBlocks(text.slice(end + 1), { quotes })
    const first = rest[0]
    if (first?.kind === 'text')
      return [{ kind: 'text', text: `${head}\n${first.text}` }, ...rest.slice(1)]
    return [{ kind: 'text', text: head }, ...rest]
  }
  const segments = splitCodeBlocks(text)
  const blocks: MessageBlock[] = []
  for (let index = 0; index < segments.length; index++) {
    const segment = segments[index]!
    if (segment.kind === 'code') {
      blocks.push({ kind: 'code', code: stripFormatting(segment.code), lang: segment.lang })
      continue
    }
    let part = segment.raw
    const nextIsCode = segments[index + 1]?.kind === 'code'
    if (segments[index - 1]?.kind === 'code') part = part.replace(/^\n/, '')
    if (nextIsCode) part = part.replace(/\n$/, '')
    if (!quotes) {
      blocks.push(...splitLists(part))
      continue
    }
    for (const piece of splitQuotes(part)) {
      if (piece.kind === 'text') {
        blocks.push(...splitLists(piece.text))
        continue
      }
      let inner = piece.text
      if (piece.rest && nextIsCode) {
        // The rest of the message: the newline trimmed above and every later segment.
        inner += `\n${segments
          .slice(index + 1)
          .map((later) => later.raw)
          .join('')}`
        index = segments.length
      }
      blocks.push({ kind: 'quote', blocks: messageBlocks(inner, { quotes: false }) })
    }
  }
  return blocks
}
