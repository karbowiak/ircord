// Markdown lists in message text, and turning pasted lists into that Markdown. Lists travel on
// IRC exactly as typed (`- item`, `1. item`), so other clients read plain Markdown and every
// IRCord view renders them from the same text, as with fenced code.

export interface MessageListItem {
  /** The item's text after its marker, IRC formatting codes included. */
  text: string
  /** Lists indented under this item. */
  children: MessageList[]
}
export interface MessageList {
  ordered: boolean
  /** The first number of an ordered list. */
  start: number
  items: MessageListItem[]
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
 * indentation nests under the item above. A line indented past the list's own indentation
 * continues the item above it; any other line ends the list, and a change between
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
      const indent = indentOf(/^[ \t]*/.exec(line)![0])
      if (stack.length && line.trim() && indent > stack[0]!.indent) {
        while (stack.length > 1 && indent <= stack.at(-1)!.indent) stack.pop()
        const item = stack.at(-1)!.list.items.at(-1)!
        item.text += `\n${line.trimStart()}`
        continue
      }
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

const skipped = new Set([
  'SCRIPT',
  'STYLE',
  'TEMPLATE',
  'HEAD',
  'TITLE',
  'META',
  'NOSCRIPT',
  'SELECT',
  'TEXTAREA',
])
/** Phrasing elements flow within a line; every other element starts a new one. */
const inlineTags = new Set([
  'A',
  'ABBR',
  'B',
  'BDI',
  'BDO',
  'BUTTON',
  'CITE',
  'CODE',
  'DATA',
  'DEL',
  'DFN',
  'EM',
  'FONT',
  'I',
  'IMG',
  'INS',
  'KBD',
  'LABEL',
  'MARK',
  'Q',
  'S',
  'SAMP',
  'SMALL',
  'SPAN',
  'STRIKE',
  'STRONG',
  'SUB',
  'SUP',
  'TIME',
  'TT',
  'U',
  'VAR',
  'WBR',
])
/** Numbers a list item can carry and still read as one (`itemLine` takes up to 9 digits). */
const maxNumber = 999_999_999
function listNumber(attribute: string | null) {
  const number = Number.parseInt(attribute ?? '', 10)
  return Number.isInteger(number) && number >= 0 && number <= maxNumber ? number : undefined
}

/**
 * Text for an HTML clipboard fragment that holds a list (copied from a web page, an editor or a
 * document), with each list item as a Markdown `- ` or `1. ` line, nested items indented two
 * spaces per level. Line breaks and paragraphs inside an item join with a space, so an item stays
 * one line and the list stays one list. Other content becomes its text, one line per block (blank
 * lines are dropped, as one would end the list). Returns null when the
 * fragment holds no list, so the browser's own plain-text paste applies. Only reads the parsed
 * nodes; nothing is rendered or fetched.
 */
export function clipboardListText(root: ParentNode): string | null {
  if (!root.querySelector('ul > li, ol > li')) return null
  const out: string[] = []
  let prefix = ''
  let content = ''
  let pre = 0
  /** How many list items the walk is inside. */
  let inItem = 0
  const breakLine = () => {
    const line = content.replace(/[ \t]+$/, '')
    if (line.trim()) {
      if (prefix || !inItem || !out.length) out.push(prefix + line)
      else out[out.length - 1] += ` ${line.trimStart()}`
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
      const ordered = tag === 'OL'
      let number = (ordered ? listNumber(element.getAttribute('start')) : undefined) ?? 1
      for (const child of element.childNodes) {
        const childTag = child.nodeType === 1 ? (child as Element).tagName.toUpperCase() : ''
        if (childTag !== 'LI') {
          // Some editors put a nested list directly inside the outer one.
          walk(child, childTag === 'UL' || childTag === 'OL' ? depth + 1 : depth)
          continue
        }
        breakLine()
        if (ordered) number = listNumber((child as Element).getAttribute('value')) ?? number
        prefix = `${'  '.repeat(depth)}${ordered ? `${number}.` : '-'} `
        number = Math.min(number + 1, maxNumber)
        inItem++
        for (const inner of child.childNodes) walk(inner, depth + 1)
        breakLine()
        inItem--
        prefix = ''
      }
      return breakLine()
    }
    const block = !inlineTags.has(tag)
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
