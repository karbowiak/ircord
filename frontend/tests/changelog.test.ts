import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  compareVersions,
  parseChangelog,
  parseInline,
  parseNotes,
  releaseLabel,
  updateNotes,
  type Changelog,
  type Release,
} from '../shared/utils/changelog'

const release = (version: string, notes = `Notes for ${version}`): Release => ({
  version,
  name: version,
  date: '2026-11-07T22:00:00Z',
  notes,
})
const history = (...versions: string[]) => versions.map((v) => release(v))
const deployed = (version: string, buildId = 'new-build', releases?: Release[]): Changelog => ({
  buildId,
  version,
  releases: releases ?? history('v0.1.4', 'v0.1.3', 'v0.1.2', 'v0.1.1', 'v0.1.0'),
})
const versions = (notes: { releases: Release[] }) => notes.releases.map((r) => r.version)

test('versions order semantically, prereleases before their release', () => {
  const sorted = ['v0.10.0', 'v0.2.0', 'v0.2.0-rc.2', 'v0.2.0-rc.10', 'v1.0.0', 'v0.9.9', 'dev']
  sorted.sort(compareVersions)
  assert.deepEqual(sorted, [
    'dev',
    'v0.2.0-rc.2',
    'v0.2.0-rc.10',
    'v0.2.0',
    'v0.9.9',
    'v0.10.0',
    'v1.0.0',
  ])
  assert.equal(compareVersions('v1.2.3', '1.2.3'), 0)
  assert.equal(compareVersions('v1.2.3+build.5', 'v1.2.3'), 0)
})

test('an update from the previous release shows just the deployed release', () => {
  const notes = updateNotes({ version: 'v0.1.3', buildId: 'old' }, deployed('v0.1.4'), 'new-build')
  assert.equal(notes.kind, 'upgrade')
  assert.equal(notes.target, 'v0.1.4')
  assert.deepEqual(versions(notes), ['v0.1.4'])
})

test('skipped releases are all included, newest first, and nothing past the deployed one', () => {
  const notes = updateNotes(
    { version: 'v0.1.1', buildId: 'old' },
    deployed('v0.1.3', 'new-build', history('v0.1.4', 'v0.1.3', 'v0.1.2', 'v0.1.1')),
    'new-build',
  )
  assert.equal(notes.kind, 'upgrade')
  assert.deepEqual(versions(notes), ['v0.1.3', 'v0.1.2'])
})

test('notes from a build other than the announced one are stale (mid-rollout replica)', () => {
  const notes = updateNotes(
    { version: 'v0.1.3', buildId: 'old' },
    deployed('v0.1.3', 'old'),
    'new-build',
  )
  assert.equal(notes.kind, 'stale')
  assert.deepEqual(notes.releases, [])
})

test('a rollback names the older release and shows only its notes', () => {
  const notes = updateNotes({ version: 'v0.1.4', buildId: 'old' }, deployed('v0.1.2'), 'new-build')
  assert.equal(notes.kind, 'rollback')
  assert.deepEqual(versions(notes), ['v0.1.2'])
})

test('a rebuild of the running release shows its own notes', () => {
  const notes = updateNotes({ version: 'v0.1.4', buildId: 'old' }, deployed('v0.1.4'), 'new-build')
  assert.equal(notes.kind, 'same')
  assert.deepEqual(versions(notes), ['v0.1.4'])
})

test('a development build cannot know the range and shows only the target', () => {
  const notes = updateNotes({ version: '', buildId: 'old' }, deployed('v0.1.4'), 'new-build')
  assert.equal(notes.kind, 'unknown-running')
  assert.deepEqual(versions(notes), ['v0.1.4'])
})

test('missing notes, unknown deployed versions and failed loads are unavailable', () => {
  const running = { version: 'v0.1.3', buildId: 'old' }
  assert.equal(updateNotes(running, null, 'new-build').kind, 'unavailable')
  assert.equal(updateNotes(running, deployed('', 'new-build', []), 'new-build').kind, 'unavailable')
  assert.equal(
    updateNotes(running, deployed('v0.1.4', 'new-build', []), 'new-build').kind,
    'unavailable',
  )
  // The deployed release itself is missing from its data.
  assert.equal(
    updateNotes(running, deployed('v0.1.4', 'new-build', history('v0.1.3')), 'new-build').kind,
    'unavailable',
  )
})

test('the payload is validated and sorted, never trusted', () => {
  assert.equal(parseChangelog(null), null)
  assert.equal(parseChangelog({ version: 'v1.0.0' }), null)
  const parsed = parseChangelog({
    buildId: 'b',
    version: 'v0.1.4',
    releases: [
      release('v0.1.2'),
      { version: 'main', notes: 'x' },
      7,
      release('v0.1.4'),
      { version: 'v0.1.3' },
    ],
  })
  assert.deepEqual(
    parsed?.releases.map((r) => r.version),
    ['v0.1.4', 'v0.1.3', 'v0.1.2'],
  )
  assert.equal(parsed?.releases[1]!.name, 'v0.1.3')
  assert.equal(parsed?.releases[1]!.notes, '')
  assert.deepEqual(parseChangelog({ buildId: 'b', releases: 'nope' })?.releases, [])
})

test('history labels the running, available and in-between releases', () => {
  assert.equal(releaseLabel('v0.1.2', 'v0.1.2', 'v0.1.4'), 'running')
  assert.equal(releaseLabel('v0.1.4', 'v0.1.2', 'v0.1.4'), 'available')
  assert.equal(releaseLabel('v0.1.3', 'v0.1.2', 'v0.1.4'), 'new')
  assert.equal(releaseLabel('v0.1.1', 'v0.1.2', 'v0.1.4'), '')
  assert.equal(releaseLabel('v0.1.4', 'v0.1.4', 'v0.1.4'), 'running')
  assert.equal(releaseLabel('v0.1.3', '', 'v0.1.4'), '')
  // Nothing past the deployed release is "not installed yet".
  assert.equal(releaseLabel('v0.1.5', 'v0.1.2', 'v0.1.4'), '')
  // No offered build: nothing is available or pending.
  assert.equal(releaseLabel('v0.1.3', 'v0.1.2', ''), '')
})

test('generated GitHub notes parse into a heading, a list with short links, and a paragraph', () => {
  const blocks = parseNotes(
    "## What's Changed\n* Add /gif <search terms> to send a GIF by @ppetermann in https://github.com/karbowiak/ircord/pull/67\n\n\n**Full Changelog**: https://github.com/karbowiak/ircord/compare/v0.1.3...v0.1.4\n<!-- generated -->",
  )
  assert.deepEqual(blocks[0], { type: 'heading', level: 2, content: [{ text: "What's Changed" }] })
  assert.deepEqual(blocks[1], {
    type: 'list',
    ordered: false,
    items: [
      {
        content: [
          { text: 'Add /gif <search terms> to send a GIF by @ppetermann in ' },
          { text: '#67', href: 'https://github.com/karbowiak/ircord/pull/67' },
        ],
      },
    ],
  })
  assert.deepEqual(blocks[2], {
    type: 'paragraph',
    content: [
      { text: 'Full Changelog', bold: true },
      { text: ': ' },
      {
        text: 'v0.1.3...v0.1.4',
        href: 'https://github.com/karbowiak/ircord/compare/v0.1.3...v0.1.4',
      },
    ],
  })
  assert.equal(blocks.length, 3)
})

test('hand-written notes keep nested and ordered lists, code and quotes', () => {
  const blocks = parseNotes(
    'Intro line\ncontinues.\n\n- one\n  - nested\n- two\n\n1. first\n2. second\n\n> quoted\n\n```\nmake test\n```\n\n---',
  )
  assert.deepEqual(
    blocks.map((b) => b.type),
    ['paragraph', 'list', 'list', 'quote', 'code', 'rule'],
  )
  assert.deepEqual(blocks[0], { type: 'paragraph', content: [{ text: 'Intro line continues.' }] })
  const list = blocks[1] as Extract<(typeof blocks)[number], { type: 'list' }>
  assert.equal(list.items.length, 2)
  assert.deepEqual(list.items[0]!.children, {
    type: 'list',
    ordered: false,
    items: [{ content: [{ text: 'nested' }] }],
  })
  assert.equal((blocks[2] as { ordered: boolean }).ordered, true)
  assert.deepEqual(blocks[4], { type: 'code', text: 'make test' })
})

test('a malformed escape in a compare link keeps the link and the rest of the notes', () => {
  const href = 'https://github.com/a/b/compare/v1%E0'
  assert.deepEqual(parseInline(`Fix ${href} by @x`), [
    { text: 'Fix ' },
    { text: href, href },
    { text: ' by @x' },
  ])
  assert.equal(parseNotes(`## Changes\n* Fix ${href}\n* Second`).length, 2)
})

test('only http(s) links become links; markup stays text', () => {
  assert.deepEqual(parseInline('[click](javascript:alert(1))'), [
    { text: '[click](javascript:alert(1))' },
  ])
  assert.deepEqual(parseInline('[docs](https://example.org/a)'), [
    { text: 'docs', href: 'https://example.org/a' },
  ])
  assert.deepEqual(parseInline('<img src=x onerror=alert(1)>'), [
    { text: '<img src=x onerror=alert(1)>' },
  ])
  assert.deepEqual(parseInline('see https://example.org/x.'), [
    { text: 'see ' },
    { text: 'https://example.org/x', href: 'https://example.org/x' },
    { text: '.' },
  ])
  assert.deepEqual(parseInline('`code` and *it* and snake_case_name'), [
    { text: 'code', code: true },
    { text: ' and ' },
    { text: 'it', italic: true },
    { text: ' and snake_case_name' },
  ])
})
