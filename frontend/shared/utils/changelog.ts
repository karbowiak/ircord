// Release notes shown inside the client. The frontend image carries the notes of the release it
// was built from and of every earlier release (see scripts/release-changelog.mjs), so what a user
// reads always describes what is actually deployed, never a newer tag that has not rolled out.

export interface Release {
  /** The git tag, such as `v0.1.4`. */
  version: string
  /** The release title; often just the tag. */
  name: string
  /** ISO time the release was published; for a draft, when it was created; otherwise when its image was built. */
  date: string
  /** Release notes as GitHub Markdown. Empty when the release has none. */
  notes: string
}

/** What `/api/app/changelog` answers: the build the server runs and its release history. */
export interface Changelog {
  /** Nuxt build id of the build serving this data; matches `/_nuxt/builds/latest.json`. */
  buildId: string
  /** The release tag that build was made from, or '' for a development build. */
  version: string
  /** Newest first. Empty when the build has no release data. */
  releases: Release[]
}

/** Parses the payload defensively: a malformed answer is treated as no data, never thrown. */
export function parseChangelog(raw: unknown): Changelog | null {
  if (!raw || typeof raw !== 'object') return null
  const { buildId, version, releases } = raw as Record<string, unknown>
  if (typeof buildId !== 'string') return null
  const list = Array.isArray(releases)
    ? releases.filter(
        (r): r is Release =>
          !!r &&
          typeof r === 'object' &&
          typeof (r as Release).version === 'string' &&
          parseVersion((r as Release).version) !== null,
      )
    : []
  return {
    buildId,
    version: typeof version === 'string' ? version : '',
    releases: list
      .map((r) => ({
        version: r.version,
        name: typeof r.name === 'string' && r.name ? r.name : r.version,
        date: typeof r.date === 'string' ? r.date : '',
        notes: typeof r.notes === 'string' ? r.notes : '',
      }))
      .sort((a, b) => compareVersions(b.version, a.version)),
  }
}

interface Version {
  core: [number, number, number]
  pre: string[]
}

/** `v1.2.3`, `1.2.3-rc.1`, `v1.2.3+build`; anything else (a branch name, '') is not a version. */
export function parseVersion(tag: string): Version | null {
  const match = /^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$/.exec(tag)
  if (!match) return null
  return {
    core: [Number(match[1]), Number(match[2]), Number(match[3])],
    pre: match[4] ? match[4].split('.') : [],
  }
}

/** Semantic-version order: negative when `a` is older than `b`. Non-versions sort oldest. */
export function compareVersions(a: string, b: string): number {
  const x = parseVersion(a)
  const y = parseVersion(b)
  if (!x || !y) return (x ? 1 : 0) - (y ? 1 : 0)
  for (let i = 0; i < 3; i++) if (x.core[i] !== y.core[i]) return x.core[i]! - y.core[i]!
  // A prerelease comes before its release.
  if (!x.pre.length || !y.pre.length) return y.pre.length - x.pre.length
  for (let i = 0; i < Math.max(x.pre.length, y.pre.length); i++) {
    const p = x.pre[i]
    const q = y.pre[i]
    if (p === undefined || q === undefined) return p === undefined ? -1 : 1
    if (p === q) continue
    const pn = /^\d+$/.test(p)
    const qn = /^\d+$/.test(q)
    if (pn && qn) return Number(p) - Number(q)
    if (pn !== qn) return pn ? -1 : 1
    return p < q ? -1 : 1
  }
  return 0
}

/** How the build on offer relates to the one this page runs. */
export type UpdateKind =
  /** Newer release: `releases` holds everything after the running one, newest first. */
  | 'upgrade'
  /** The same release rebuilt (or a development build is running): only its own notes. */
  | 'same'
  /** An older release is deployed again. */
  | 'rollback'
  /** The running build has no release version, so the range cannot be known. */
  | 'unknown-running'
  /** The deployed build carries no release data. */
  | 'unavailable'
  /** The answer came from a different build than the one announced (mid-rollout); try again. */
  | 'stale'

export interface UpdateNotes {
  kind: UpdateKind
  /** The deployed release, when known. */
  target: string
  releases: Release[]
}

/**
 * Picks the notes to show for an announced update. `running` is the release this page was built
 * from ('' for a development build); `announcedId` is the build id the manifest reported, so notes
 * from a replica that serves some other build are never presented as the update.
 */
export function updateNotes(
  running: { version: string; buildId: string },
  changelog: Changelog | null,
  announcedId: string,
): UpdateNotes {
  if (!changelog) return { kind: 'unavailable', target: '', releases: [] }
  if (announcedId && changelog.buildId !== announcedId)
    return { kind: 'stale', target: '', releases: [] }
  const target = changelog.version
  const own = changelog.releases.find((r) => r.version === target)
  if (!parseVersion(target) || !own) return { kind: 'unavailable', target, releases: [] }
  if (!parseVersion(running.version)) return { kind: 'unknown-running', target, releases: [own] }
  const order = compareVersions(target, running.version)
  if (order < 0) return { kind: 'rollback', target, releases: [own] }
  if (order === 0) return { kind: 'same', target, releases: [own] }
  return {
    kind: 'upgrade',
    target,
    releases: changelog.releases.filter(
      (r) =>
        compareVersions(r.version, running.version) > 0 && compareVersions(r.version, target) <= 0,
    ),
  }
}

/**
 * Where a release in the history stands for this page: the one it runs, the one the server now
 * offers, or one in between that a reload would also bring.
 */
export function releaseLabel(
  version: string,
  running: string,
  deployed: string,
): 'running' | 'available' | 'new' | '' {
  if (version === running) return 'running'
  if (version === deployed) return 'available'
  if (
    parseVersion(running) &&
    compareVersions(version, running) > 0 &&
    compareVersions(version, deployed) < 0
  )
    return 'new'
  return ''
}

// ---- Markdown ---------------------------------------------------------------------------------
// Release notes are GitHub Markdown written by maintainers or generated from pull request titles,
// which may contain arbitrary text. They are parsed into plain data and rendered with Vue text
// bindings, never as HTML, so nothing in them can inject markup or script.

/** A run of inline text; `href` is set only for http(s) links. */
export interface Inline {
  text: string
  bold?: boolean
  italic?: boolean
  code?: boolean
  href?: string
}

export interface ListItem {
  content: Inline[]
  children?: NoteBlock
}

export type NoteBlock =
  | { type: 'heading'; level: number; content: Inline[] }
  | { type: 'paragraph'; content: Inline[] }
  | { type: 'list'; ordered: boolean; items: ListItem[] }
  | { type: 'code'; text: string }
  | { type: 'quote'; content: Inline[] }
  | { type: 'rule' }

/** Only web links are followed; `javascript:` and other schemes stay plain text. */
export function safeHref(url: string): string | undefined {
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'https:' || parsed.protocol === 'http:' ? parsed.href : undefined
  } catch {
    return undefined
  }
}

/** GitHub's own short forms for links that would otherwise print as long URLs. */
function linkLabel(href: string): string {
  const pull = /^https:\/\/github\.com\/[^/]+\/[^/]+\/(?:pull|issues)\/(\d+)\/?$/.exec(href)
  if (pull) return `#${pull[1]}`
  const compare = /^https:\/\/github\.com\/[^/]+\/[^/]+\/compare\/([^/?#]+)$/.exec(href)
  if (compare) {
    try {
      return decodeURIComponent(compare[1]!)
    } catch {
      // A malformed %-escape (from any merged pull request title) must not break the notes.
      return href
    }
  }
  return href
}

const INLINE =
  /(`[^`]+`)|\[([^\]]+)\]\(([^)\s]+)\)|(https?:\/\/[^\s<>()]*[^\s<>().,;:!?'"])|(\*\*|__)(.+?)\5|(?<![\w*])([*_])(?!\s)(.+?)(?<!\s)\7(?![\w*])/g

/** Splits a line into styled runs: code, links, bare URLs, bold and italic. */
export function parseInline(text: string, style: Omit<Inline, 'text'> = {}): Inline[] {
  const out: Inline[] = []
  const push = (run: Inline) => {
    if (!run.text) return
    const last = out[out.length - 1]
    if (
      last &&
      !last.href &&
      !run.href &&
      !!last.bold === !!run.bold &&
      !!last.italic === !!run.italic &&
      !!last.code === !!run.code
    )
      last.text += run.text
    else out.push(run)
  }
  let at = 0
  for (const match of text.matchAll(INLINE)) {
    push({ ...style, text: text.slice(at, match.index) })
    at = match.index + match[0].length
    const [whole, code, label, url, bare, , strong, , emphasis] = match
    if (code) push({ ...style, code: true, text: code.slice(1, -1) })
    else if (label !== undefined) {
      const href = safeHref(url!)
      if (href) out.push({ ...style, text: label, href })
      else push({ ...style, text: whole })
    } else if (bare) {
      const href = safeHref(bare)
      if (href) out.push({ ...style, text: linkLabel(href), href })
      else push({ ...style, text: bare })
    } else if (strong !== undefined)
      for (const run of parseInline(strong, { ...style, bold: true })) push(run)
    else if (emphasis !== undefined)
      for (const run of parseInline(emphasis, { ...style, italic: true })) push(run)
  }
  push({ ...style, text: text.slice(at) })
  return out
}

const LIST_ITEM = /^(\s*)([-*+]|\d+[.)])\s+(.*)$/

/** Parses release-note Markdown into blocks. Raw HTML is shown as text; comments are dropped. */
export function parseNotes(markdown: string): NoteBlock[] {
  const lines = markdown
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\r\n?/g, '\n')
    .split('\n')
  const blocks: NoteBlock[] = []
  let paragraph: string[] = []
  const flush = () => {
    if (paragraph.length)
      blocks.push({ type: 'paragraph', content: parseInline(paragraph.join(' ')) })
    paragraph = []
  }
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]!
    const trimmed = line.trim()
    if (!trimmed) {
      flush()
      continue
    }
    if (trimmed.startsWith('```')) {
      flush()
      const code: string[] = []
      while (++i < lines.length && !lines[i]!.trim().startsWith('```')) code.push(lines[i]!)
      blocks.push({ type: 'code', text: code.join('\n') })
      continue
    }
    const heading = /^(#{1,6})\s+(.*?)\s*#*$/.exec(trimmed)
    if (heading) {
      flush()
      blocks.push({ type: 'heading', level: heading[1]!.length, content: parseInline(heading[2]!) })
      continue
    }
    if (/^([-*_])(\s*\1){2,}$/.test(trimmed)) {
      flush()
      blocks.push({ type: 'rule' })
      continue
    }
    if (trimmed.startsWith('>')) {
      flush()
      blocks.push({ type: 'quote', content: parseInline(trimmed.replace(/^>\s?/, '')) })
      continue
    }
    if (LIST_ITEM.test(line)) {
      flush()
      const items: { indent: number; ordered: boolean; text: string }[] = []
      for (; i < lines.length; i++) {
        const item = LIST_ITEM.exec(lines[i]!)
        if (item)
          items.push({
            indent: item[1]!.replace(/\t/g, '  ').length,
            ordered: /\d/.test(item[2]!),
            text: item[3]!,
          })
        else if (lines[i]!.trim() && /^\s+/.test(lines[i]!) && items.length)
          items[items.length - 1]!.text += ` ${lines[i]!.trim()}`
        else break
      }
      i--
      blocks.push(nestList(items))
      continue
    }
    paragraph.push(trimmed)
  }
  flush()
  return blocks
}

function nestList(items: { indent: number; ordered: boolean; text: string }[]): NoteBlock {
  const root: NoteBlock & { type: 'list' } = {
    type: 'list',
    ordered: items[0]!.ordered,
    items: [],
  }
  const stack: { indent: number; list: NoteBlock & { type: 'list' } }[] = [
    { indent: items[0]!.indent, list: root },
  ]
  for (const item of items) {
    while (stack.length > 1 && item.indent < stack[stack.length - 1]!.indent) stack.pop()
    let top = stack[stack.length - 1]!
    const parent = top.list.items[top.list.items.length - 1]
    if (item.indent > top.indent && parent) {
      const list: NoteBlock & { type: 'list' } = { type: 'list', ordered: item.ordered, items: [] }
      parent.children = list
      stack.push({ indent: item.indent, list })
      top = stack[stack.length - 1]!
    }
    top.list.items.push({ content: parseInline(item.text) })
  }
  return root
}
