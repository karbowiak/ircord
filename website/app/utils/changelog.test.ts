import { strict as assert } from 'node:assert'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { formatDate, parseChangelog, parseCollaborators, parseInline } from './changelog'

const sample = `# Changelog

Intro with **New**.

## Unreleased

### New

- Not released yet.

## v1.2.0 — 2026-11-17

<!-- hidden -->

### New

- First item with **bold**, \`code\` and [a link](https://example.com/x).
- Second item
  continues here.

### Fixed

A paragraph
over two lines.

## v1.1.0

## v1.1.0-rc.1 – 2026-11-10

- A prerelease item.

## v1.0.0 — 2026-11-01

- Item before any group.

\`\`\`
## v9.9.9 (inside a fence)
\`\`\`
`

test('reads releases newest first, without Unreleased, empty releases or fenced headings', () => {
  assert.deepEqual(
    parseChangelog(sample).map((release) => [release.version, release.date]),
    [
      ['v1.2.0', '2026-11-17'],
      ['v1.1.0-rc.1', '2026-11-10'],
      ['v1.0.0', '2026-11-01'],
    ],
  )
})

test('reads groups, list items, wrapped lines and paragraphs', () => {
  const [latest, , first] = parseChangelog(sample)
  assert.deepEqual(
    latest!.groups.map((group) => group.title),
    ['New', 'Fixed'],
  )
  const list = latest!.groups[0]!.blocks[0]!
  assert.equal(list.type, 'list')
  if (list.type !== 'list') return
  assert.equal(list.items.length, 2)
  assert.equal(list.items[1]!.map((part) => part.text).join(''), 'Second item continues here.')
  assert.deepEqual(latest!.groups[1]!.blocks[0], {
    type: 'paragraph',
    content: [{ type: 'text', text: 'A paragraph over two lines.' }],
  })
  // Items before the first group get an untitled group.
  assert.equal(first!.groups[0]!.title, '')
})

test('stops at a release heading in another format', () => {
  assert.throws(() => parseChangelog('## v0.7.0 (2026-11-18)\n\n- Lost.\n'), /v0\.7\.0/)
})

test('splits bold, code and https links, and keeps everything else as text', () => {
  assert.deepEqual(parseInline('a **b** `c` [d](https://e.test) <i>f</i> [g](javascript:x)'), [
    { type: 'text', text: 'a ' },
    { type: 'strong', text: 'b' },
    { type: 'text', text: ' ' },
    { type: 'code', text: 'c' },
    { type: 'text', text: ' ' },
    { type: 'link', text: 'd', href: 'https://e.test' },
    { type: 'text', text: ' <i>f</i> [g](javascript:x)' },
  ])
})

test('reads the repository changelog and collaborators', () => {
  const releases = parseChangelog(
    readFileSync(new URL('../../../CHANGELOG.md', import.meta.url), 'utf8'),
  )
  assert.ok(releases.length > 0)
  assert.ok(releases.every((release) => release.groups.length > 0))
  const names = parseCollaborators(
    readFileSync(new URL('../../../COLLABORATORS.md', import.meta.url), 'utf8'),
  )
  assert.ok(names.length > 0)
})

test('reads collaborators from list lines only', () => {
  assert.deepEqual(parseCollaborators('# T\n\nText line.\n\n- Ada Lovelace\n- Lin\n'), [
    'Ada Lovelace',
    'Lin',
  ])
})

test('formats dates in US English, independent of the time zone', () => {
  assert.equal(formatDate('2026-11-17'), 'November 17, 2026')
  assert.equal(formatDate(''), '')
})
