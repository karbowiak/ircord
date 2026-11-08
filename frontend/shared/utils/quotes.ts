// Discord-style quote blocks in message text. Quotes travel on IRC exactly as typed (`> line`,
// `>>> rest`), so other clients read the plain marker and every IRCord view renders them from the
// same text, as with lists and fenced code.

export type QuoteBlock =
  | { kind: 'text'; text: string }
  /** `rest` marks a `>>> ` quote, which runs to the end of the message. */
  | { kind: 'quote'; text: string; rest?: true }

const quoteLine = /^> (.*)$/
const restMarker = /^>>> (.*)$/
const anyQuote = /(?:^|\n)>(?:>>)? /

/**
 * Splits text into ordinary text and quotes. A line that starts with `> ` is quoted, and
 * consecutive quoted lines form one quote. A line that starts with `>>> ` quotes itself and every
 * line after it. `>` without a following space, or anywhere but the start of a line, stays text.
 * A quote of blank lines only stays text, as typed, and a `>>> ` with nothing after it on its line
 * starts the quote on the next line. The line breaks around a quote belong to the quote. Linear in
 * the input.
 */
export function splitQuotes(text: string): QuoteBlock[] {
  if (!anyQuote.test(text)) return [{ kind: 'text', text }]
  const blocks: QuoteBlock[] = []
  let lines: string[] = []
  /** The `> ` lines being collected, without their markers. */
  let quoted: string[] = []
  const blank = (texts: string[]) => !texts.some((line) => line.trim())
  const endQuote = () => {
    if (!quoted.length) return
    if (blank(quoted)) lines.push(...quoted.map((line) => `> ${line}`))
    else {
      if (lines.length) blocks.push({ kind: 'text', text: lines.join('\n') })
      blocks.push({ kind: 'quote', text: quoted.join('\n') })
      lines = []
    }
    quoted = []
  }
  const all = text.split('\n')
  for (let i = 0; i < all.length; i++) {
    const line = all[i]!
    const rest = restMarker.exec(line)
    const after = all.slice(i + 1)
    if (rest && !blank([rest[1]!, ...after])) {
      endQuote()
      if (lines.length) blocks.push({ kind: 'text', text: lines.join('\n') })
      const first = rest[1]!.trim() ? [rest[1]!] : []
      blocks.push({ kind: 'quote', text: [...first, ...after].join('\n'), rest: true })
      return blocks
    }
    const match = quoteLine.exec(line)
    if (match) quoted.push(match[1]!)
    else {
      endQuote()
      lines.push(line)
    }
  }
  endQuote()
  if (lines.length) blocks.push({ kind: 'text', text: lines.join('\n') })
  return blocks
}
