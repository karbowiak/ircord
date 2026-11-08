// Markdown lists in message text, and turning pasted lists into that Markdown. Lists travel on
// IRC exactly as typed (`- item`, `1. item`), so other clients read plain Markdown and every
// IRCord view renders them from the same text, as with fenced code.

export interface ListItem {
  /** The item's text after its marker, IRC formatting codes included. */
  text: string
  /** Lists indented under this item. */
  children: MessageList[]
}
export interface MessageList {
  ordered: boolean
  /** The first number of an ordered list. */
  start: number
  items: ListItem[]
}
export type ListBlock = { kind: 'text'; text: string } | { kind: 'list'; list: MessageList }

/** Deeper indentation than this nests no further, so hostile input stays shallow. */
export const maxListDepth = 5

const itemLine = /^([ \t]*)([-*+]|(\d{1,9})[.)])[ \t]+(?=\S)(.*)$/
const anyItem = /(?:^|\n)[ \t]*(?:[-*+]|\d{1,9}[.)])[ \t]+\S/

const indentOf = (space: string) => space.replace(/\t/g, '    ').length

/**
 * Splits text into ordinary text and lists. A list item is a line that starts (after optional
 * indentation) with `-`, `*`, `+`, `1.` or `1)` and a space, followed by text; deeper
 * indentation nests under the item above. Any other line ends the list, and a change between
 * bulleted and numbered items at the same level starts a new list. The line breaks around a list
 * belong to the list. Linear in the input.
 */
export function splitLists(text: string): ListBlock[] {
  if (!anyItem.test(text)) return [{ kind: 'text', text }]
  const blocks: ListBlock[] = []
  let lines: string[] = []
  /** The open lists, outermost first, with the indentation their items start at. */
  let stack: { indent: number; list: MessageList }[] = []
  const flushText = () => {
    if (lines.length) blocks.push({ kind: 'text', text: lines.join('\n') })
    lines = []
  }
  const open = (indent: number, ordered: boolean, start: number) => {
    const list: MessageList = { ordered, start, items: [] }
    const parent = stack.at(-1)?.list.items.at(-1)
    if (parent) parent.children.push(list)
    else blocks.push({ kind: 'list', list })
    stack.push({ indent, list })
    return list
  }
  for (const line of text.split('\n')) {
    const match = itemLine.exec(line)
    if (!match) {
      stack = []
      lines.push(line)
      continue
    }
    flushText()
    const indent = indentOf(match[1]!)
    const ordered = match[3] !== undefined
    const number = ordered ? Number(match[3]) : 1
    while (stack.length > 1 && indent < stack.at(-1)!.indent) stack.pop()
    let level = stack.at(-1)
    if (level && indent > level.indent && stack.length < maxListDepth) level = undefined
    else if (level?.list.ordered !== ordered && level) {
      stack.pop()
      level = undefined
    }
    const list = level?.list ?? open(indent, ordered, number)
    list.items.push({ text: match[4]!, children: [] })
  }
  flushText()
  return blocks
}

// ---- Pasting ----------------------------------------------------------------------------------

const bulletGlyph = /^([ \t]*)[•◦▪▫‣⁃●○■□][ \t]+(?=\S)/gm

/** Plain text whose lines start with bullet glyphs (•, ◦, ▪ …) gets Markdown `- ` markers. */
export function normalizeBullets(text: string) {
  return text.replace(bulletGlyph, '$1- ')
}

const skipped = new Set(['SCRIPT', 'STYLE', 'TEMPLATE', 'HEAD', 'TITLE', 'META', 'NOSCRIPT'])
const blockTags = new Set([
  'ADDRESS',
  'ARTICLE',
  'ASIDE',
  'BLOCKQUOTE',
  'DD',
  'DIV',
  'DL',
  'DT',
  'FIGCAPTION',
  'FIGURE',
  'FOOTER',
  'H1',
  'H2',
  'H3',
  'H4',
  'H5',
  'H6',
  'HEADER',
  'HR',
  'LI',
  'MAIN',
  'NAV',
  'P',
  'PRE',
  'SECTION',
  'TABLE',
  'TR',
])

/**
 * Text for an HTML clipboard fragment that holds a list (copied from a web page, an editor or a
 * document), with each list item as a Markdown `- ` or `1. ` line, nested items indented two
 * spaces per level. Other content becomes its text, one line per block (blank lines are dropped,
 * as one would end the list). Returns null when the
 * fragment holds no list, so the browser's own plain-text paste applies. Only reads the parsed
 * nodes; nothing is rendered or fetched.
 */
export function clipboardListText(root: ParentNode): string | null {
  if (!root.querySelector('ul > li, ol > li')) return null
  const out: string[] = []
  let prefix = ''
  let content = ''
  let pre = 0
  const breakLine = () => {
    const line = content.replace(/[ \t]+$/, '')
    if (line.trim()) {
      out.push(prefix + line)
      prefix = ''
    }
    content = ''
  }
  const walk = (node: Node, depth: number) => {
    if (node.nodeType === 3) {
      let text = node.textContent ?? ''
      if (!pre) {
        text = text.replace(/\s+/g, ' ')
        if (!content) text = text.trimStart()
      }
      if (pre) {
        const [first, ...rest] = text.split('\n')
        content += first
        for (const line of rest) {
          breakLine()
          content = line
        }
      } else content += text
      return
    }
    if (node.nodeType !== 1) return
    const element = node as Element
    const tag = element.tagName.toUpperCase()
    if (skipped.has(tag)) return
    if (tag === 'BR') return breakLine()
    if (tag === 'UL' || tag === 'OL') {
      breakLine()
      let number = tag === 'OL' ? Number(element.getAttribute('start')) || 1 : 1
      for (const child of element.childNodes) {
        if (child.nodeType !== 1 || (child as Element).tagName.toUpperCase() !== 'LI') {
          walk(child, depth)
          continue
        }
        breakLine()
        const value = Number((child as Element).getAttribute('value'))
        if (tag === 'OL' && value) number = value
        prefix = `${'  '.repeat(depth)}${tag === 'OL' ? `${number++}.` : '-'} `
        for (const inner of child.childNodes) walk(inner, depth + 1)
        breakLine()
        prefix = ''
      }
      return breakLine()
    }
    const block = blockTags.has(tag)
    if (block) breakLine()
    if (tag === 'PRE') pre++
    for (const child of element.childNodes) walk(child, depth)
    if (tag === 'PRE') pre--
    if (block) breakLine()
  }
  walk(root as unknown as Node, 0)
  breakLine()
  return normalizeBullets(out.join('\n'))
}
