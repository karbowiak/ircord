/**
 * Reads CHANGELOG.md and COLLABORATORS.md into data the pages render. Only the Markdown the
 * changelog uses is understood: `##` release headings, `###` groups, `-` list items and
 * paragraphs, with **bold**, `code` and [links](https://…) inside them. Text is never treated
 * as HTML.
 */

export type Inline =
  | { type: 'text'; text: string }
  | { type: 'strong'; text: string }
  | { type: 'code'; text: string }
  | { type: 'link'; text: string; href: string }

export type Block = { type: 'paragraph'; content: Inline[] } | { type: 'list'; items: Inline[][] }

export interface ReleaseGroup {
  /** The `###` heading, such as New, Improved or Fixed; empty for text before the first one. */
  title: string
  blocks: Block[]
}

export interface Release {
  version: string
  /** `YYYY-MM-DD`, or empty when the heading has none. */
  date: string
  groups: ReleaseGroup[]
}

/** `## v1.2.3 — 2026-11-08`; `## Unreleased` and other headings are not releases. */
const RELEASE = /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\s+[-–—]+\s+(\d{4}-\d{2}-\d{2}))?\s*$/
const INLINE = /\*\*(.+?)\*\*|`([^`]+)`|\[([^\]]+)\]\((https:\/\/[^\s)]+)\)/g

export function parseInline(text: string): Inline[] {
  const out: Inline[] = []
  let last = 0
  for (const match of text.matchAll(INLINE)) {
    if (match.index > last) out.push({ type: 'text', text: text.slice(last, match.index) })
    if (match[1] !== undefined) out.push({ type: 'strong', text: match[1] })
    else if (match[2] !== undefined) out.push({ type: 'code', text: match[2] })
    else out.push({ type: 'link', text: match[3]!, href: match[4]! })
    last = match.index + match[0].length
  }
  if (last < text.length) out.push({ type: 'text', text: text.slice(last) })
  return out
}

/** The releases in CHANGELOG.md, in file order (newest first). Releases with no notes are left out. */
export function parseChangelog(markdown: string): Release[] {
  // Blocks keep their raw text until the end, so wrapped lines join before inline parsing.
  type RawBlock = { type: 'paragraph'; text: string } | { type: 'list'; items: string[] }
  type RawRelease = Omit<Release, 'groups'> & { groups: { title: string; blocks: RawBlock[] }[] }
  const releases: RawRelease[] = []
  let release: RawRelease | undefined
  let group: RawRelease['groups'][number] | undefined
  let block: RawBlock | undefined
  let fence = false

  const lines = markdown
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\r\n?/g, '\n')
    .split('\n')
  for (const raw of lines) {
    const line = raw.trimEnd()
    if (/^\s*```/.test(line)) fence = !fence
    const heading = fence ? null : /^(#{2,3})\s+(.*)$/.exec(line)
    if (heading?.[1] === '##') {
      const match = RELEASE.exec(heading[2]!)
      // A release heading in another format would drop that release from the page; stop the build.
      if (!match && /^v\d/.test(heading[2]!))
        throw new Error(
          `CHANGELOG.md: "## ${heading[2]}" is not a release heading such as "## v1.2.3 — 2026-11-17".`,
        )
      release = match
        ? { version: heading[2]!.split(/\s/)[0]!, date: match[1] ?? '', groups: [] }
        : undefined
      if (release) releases.push(release)
      group = block = undefined
      continue
    }
    if (!release) continue
    if (heading) {
      group = { title: heading[2]!.trim(), blocks: [] }
      release.groups.push(group)
      block = undefined
      continue
    }
    if (!line.trim()) {
      block = undefined
      continue
    }
    if (!group) {
      group = { title: '', blocks: [] }
      release.groups.push(group)
    }
    const item = /^\s*[-*]\s+(.*)$/.exec(line)
    if (item) {
      if (block?.type !== 'list') {
        block = { type: 'list', items: [] }
        group.blocks.push(block)
      }
      block.items.push(item[1]!)
    } else if (block?.type === 'list' && /^\s+/.test(line)) {
      // A wrapped list item continues on an indented line.
      block.items[block.items.length - 1] += ` ${line.trim()}`
    } else if (block?.type === 'paragraph') {
      block.text += ` ${line.trim()}`
    } else {
      block = { type: 'paragraph', text: line.trim() }
      group.blocks.push(block)
    }
  }
  return releases
    .filter((entry) => entry.groups.some((g) => g.blocks.length))
    .map((entry) => ({
      ...entry,
      groups: entry.groups.map((g): ReleaseGroup => ({
        title: g.title,
        blocks: g.blocks.map((b): Block =>
          b.type === 'list'
            ? { type: 'list', items: b.items.map(parseInline) }
            : { type: 'paragraph', content: parseInline(b.text) },
        ),
      })),
    }))
}

/** The names in COLLABORATORS.md, one `- Name` line each. */
export function parseCollaborators(markdown: string): string[] {
  return [...markdown.matchAll(/^\s*[-*]\s+(.+?)\s*$/gm)].map((match) => match[1]!)
}

/** `2026-11-17` as `November 17, 2026`, independent of the build machine's locale and zone. */
export function formatDate(date: string): string {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) return date
  return new Date(`${date}T00:00:00Z`).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    timeZone: 'UTC',
  })
}
