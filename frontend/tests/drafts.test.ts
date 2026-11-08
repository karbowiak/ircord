import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  DRAFT_LIMIT,
  discardDraft,
  draftCount,
  draftPreview,
  mergeDrafts,
  draftRows,
  restoreDraft,
  sanitizeDrafts,
  sanitizeDraftTimes,
  stampDraftTimes,
} from '../shared/utils/drafts'
import type { Conversation, Network } from '../shared/types/chat'

const network = (id: string, name: string) => ({ id, name }) as Network
const conversation = (id: string, networkId: string, name: string, kind = 'channel') =>
  ({ id, networkId, name, kind, memberIds: [] }) as unknown as Conversation
const workspace = {
  networks: [network('n1', 'Libera'), network('n2', 'OFTC')],
  conversations: [
    conversation('c1', 'n1', 'linux'),
    conversation('c2', 'n2', 'linux'),
    conversation('d1', 'n1', 'jules', 'direct'),
  ],
}
const label = (c: Conversation) => c.name

test('the stored draft payload reads exactly as before', () => {
  // A legacy payload: plain text per conversation id, no metadata.
  const legacy = { c1: 'hello', c2: '', d1: 42, 'server:n1': 'WHOIS jules' }
  assert.deepEqual(sanitizeDrafts(legacy), { c1: 'hello', 'server:n1': 'WHOIS jules' })
  assert.deepEqual(sanitizeDrafts(null), {})
  assert.equal(sanitizeDrafts({ c1: 'x'.repeat(5000) }).c1?.length, 4000)
})

test('edit times are kept only for existing drafts with plausible values', () => {
  const drafts = { c1: 'a', c2: 'b', d1: 'c' }
  assert.deepEqual(sanitizeDraftTimes({ c1: 1000, c2: 'soon', d1: -5, gone: 2000 }, drafts), {
    c1: 1000,
  })
  assert.deepEqual(sanitizeDraftTimes(undefined, drafts), {})
})

test('changed drafts get a new time, untouched legacy drafts stay unknown, removed ones go', () => {
  const times: Record<string, number> = { c1: 100, c2: 200 }
  const previous = { c1: 'one', c2: 'two', d1: 'legacy' }
  const current = { c1: 'one!', d1: 'legacy' }
  assert.equal(stampDraftTimes(previous, current, times, 500), true)
  assert.deepEqual(times, { c1: 500 })
  assert.equal(stampDraftTimes(current, current, times, 900), false)
  assert.deepEqual(times, { c1: 500 })
})

test('previews are single-line, bounded and cut on a character boundary', () => {
  assert.equal(draftPreview('  line one\n\n  line two\t '), 'line one line two')
  const long = draftPreview('😀'.repeat(400), 10)
  assert.equal(Array.from(long).length, 10)
  assert.ok(long.endsWith('…'))
  assert.ok(!long.includes('�'))
  assert.equal(draftPreview('short', 10), 'short')
})

test('rows name the network so same-named channels stay apart, newest edit first', () => {
  const rows = draftRows(
    { c1: 'on libera', c2: 'on oftc', d1: 'to jules', 'server:n2': 'MOTD' },
    { c2: 300, c1: 100, 'server:n2': 200 },
    workspace,
    label,
  )
  assert.deepEqual(
    rows.map((r) => [r.id, r.title, r.networkName, r.kind, r.state]),
    [
      ['c2', 'linux', 'OFTC', 'channel', 'available'],
      ['server:n2', 'Server window', 'OFTC', 'server', 'available'],
      ['c1', 'linux', 'Libera', 'channel', 'available'],
      // A legacy draft has no time; it sorts after the timed ones instead of pretending.
      ['d1', 'jules', 'Libera', 'direct', 'available'],
    ],
  )
  assert.equal(rows[3]?.editedAt, undefined)
})

test('drafts for removed conversations and networks stay listed as unavailable', () => {
  const rows = draftRows(
    { gone: 'keep me', 'server:n9': 'QUIT', '   ': 'x', c1: '   ' },
    {},
    workspace,
    label,
  )
  assert.deepEqual(
    rows.map((r) => [r.id, r.state, r.text]),
    [
      ['server:n9', 'unavailable', 'QUIT'],
      ['   ', 'unavailable', 'x'],
      ['gone', 'unavailable', 'keep me'],
    ],
  )
})

test('before the workspace loads, rows are unknown rather than unavailable', () => {
  const rows = draftRows({ c1: 'hi' }, {}, undefined, label)
  assert.equal(rows[0]?.state, 'unknown')
  // Not called unavailable (it may well exist); the dialog adds no channel # to it either.
  assert.equal(rows[0]?.title, 'Conversation')
})

test('the list stays bounded and keeps the newest drafts, whatever order they were added', () => {
  const many = Object.fromEntries(
    Array.from({ length: DRAFT_LIMIT + 50 }, (_, i) => [`c${i}`, `draft ${i}`]),
  )
  // Whitespace-only entries neither count nor use up a slot.
  const drafts = { blank: '   ', ...many }
  const times = Object.fromEntries(Object.keys(many).map((id, i) => [id, i + 1]))
  const rows = draftRows(drafts, times, workspace, label)
  assert.equal(rows.length, DRAFT_LIMIT)
  assert.equal(draftCount(drafts), DRAFT_LIMIT + 50)
  assert.equal(rows[0]?.id, `c${DRAFT_LIMIT + 49}`)
  assert.equal(rows.at(-1)?.id, 'c50')
})

test('a stale discard never erases newer text', () => {
  const drafts: Record<string, string> = { c1: 'newer text' }
  assert.equal(discardDraft(drafts, 'c1', 'older text'), false)
  assert.deepEqual(drafts, { c1: 'newer text' })
  assert.equal(discardDraft(drafts, 'c1', 'newer text'), true)
  assert.deepEqual(drafts, {})
})

test('undo restores the text but never over a draft written since, and says why', () => {
  const drafts: Record<string, string> = {}
  assert.equal(restoreDraft(drafts, 'c1', 'old'), 'restored')
  assert.deepEqual(drafts, { c1: 'old' })
  drafts.c1 = 'typed meanwhile'
  assert.equal(restoreDraft(drafts, 'c1', 'old'), 'conflict')
  assert.deepEqual(drafts, { c1: 'typed meanwhile' })
  const full = Object.fromEntries(Array.from({ length: DRAFT_LIMIT }, (_, i) => [`d${i}`, 'x']))
  assert.equal(restoreDraft(full, 'c1', 'old'), 'full')
  assert.equal(full.c1, undefined)
})

test("merging with another tab keeps its newer text and this tab's own edits", () => {
  const base = { a: 'saved a', b: 'saved b', c: 'saved c', d: 'saved d' }
  // This tab: edited a, discarded b, left c and d alone, typed e.
  const mine = { a: 'my a', c: 'saved c', d: 'saved d', e: 'my e' }
  // Another tab: edited b and c, sent d, typed f.
  const theirs = { a: 'saved a', b: 'their b', c: 'their c', f: 'their f' }
  const { merged, adopted, seen } = mergeDrafts(base, mine, theirs)
  assert.deepEqual(merged, {
    a: 'my a',
    // Removing a draft here never erases newer text saved in another tab.
    b: 'their b',
    c: 'their c',
    e: 'my e',
    f: 'their f',
  })
  assert.deepEqual(adopted.sort(), ['b', 'c', 'd', 'f'])
  // The next merge starts from what was taken from storage, and from the old base for a and e
  // (changed here, not saved yet), so a later save there is still recognised as newer.
  assert.deepEqual(seen, { a: 'saved a', b: 'their b', c: 'their c', f: 'their f' })
})

test('a stale tab discarding an older copy keeps the newer text another tab saved', () => {
  // Tab A last saw "older"; tab B saved "newer" since. A's overview row still shows "older".
  const base = { c1: 'older' }
  const mine: Record<string, string> = { c1: 'older' }
  const theirs = { c1: 'newer' }
  const { merged } = mergeDrafts(base, mine, theirs)
  for (const [id, text] of Object.entries(merged)) mine[id] = text
  assert.equal(discardDraft(mine, 'c1', 'older'), false)
  assert.deepEqual(mine, { c1: 'newer' })
  // Even if this tab removed it first, the merge keeps the newer text.
  assert.deepEqual(mergeDrafts(base, {}, theirs).merged, { c1: 'newer' })
})
