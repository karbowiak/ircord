// Discord-style quote blocks in message text. Quotes travel on IRC exactly as typed (`> line`,
// `>>> rest`), so other clients read the plain marker and every IRCord view renders them from the
// same text, as with lists and fenced code.

export type QuoteBlock = { kind: 'text'; text: string } | { kind: 'quote'; text: string }

const quoteLine = /^> (.*)$/
const restMarker = /^>>> ([^]*)$/
const anyQuote = /(?:^|\n)>(?:>>)? /

/**
 * Splits text into ordinary text and quotes. A line that starts with `> ` is quoted, and
 * consecutive quoted lines form one quote. A line that starts with `>>> ` quotes itself and every
 * line after it. `>` without a following space, or anywhere but the start of a line, stays text.
 * The line breaks around a quote belong to the quote. Linear in the input.
 */
export function splitQuotes(text: string): QuoteBlock[] {
  if (!anyQuote.test(text)) return [{ kind: 'text', text }]
  const blocks: QuoteBlock[] = []
  let lines: string[] = []
  let quoted: string[] | null = null
  const flush = () => {
    if (quoted) blocks.push({ kind: 'quote', text: quoted.join('\n') })
    else if (lines.length) blocks.push({ kind: 'text', text: lines.join('\n') })
    lines = []
    quoted = null
  }
  const all = text.split('\n')
  for (let i = 0; i < all.length; i++) {
    const line = all[i]!
    const rest = restMarker.exec(line)
    if (rest) {
      flush()
      blocks.push({ kind: 'quote', text: [rest[1]!, ...all.slice(i + 1)].join('\n') })
      return blocks
    }
    const match = quoteLine.exec(line)
    if (match) {
      if (!quoted) flush()
      quoted ??= []
      quoted.push(match[1]!)
    } else {
      if (quoted) flush()
      lines.push(line)
    }
  }
  flush()
  return blocks
}
