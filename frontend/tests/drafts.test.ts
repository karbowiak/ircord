import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  DRAFT_LIMIT,
  discardDraft,
  draftPreview,
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
  assert.deepEqual(
    sanitizeDraftTimes({ c1: 1000, c2: 'soon', d1: -5, gone: 2000 }, drafts),
    { c1: 1000 },
  )
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
})

test('the list stays bounded', () => {
  const many = Object.fromEntries(
    Array.from({ length: DRAFT_LIMIT + 50 }, (_, i) => [`c${i}`, `draft ${i}`]),
  )
  assert.equal(draftRows(many, {}, workspace, label).length, DRAFT_LIMIT)
})

test('a stale discard never erases newer text', () => {
  const drafts: Record<string, string> = { c1: 'newer text' }
  assert.equal(discardDraft(drafts, 'c1', 'older text'), false)
  assert.deepEqual(drafts, { c1: 'newer text' })
  assert.equal(discardDraft(drafts, 'c1', 'newer text'), true)
  assert.deepEqual(drafts, {})
})

test('undo restores the text but never over a draft written since', () => {
  const drafts: Record<string, string> = {}
  assert.equal(restoreDraft(drafts, 'c1', 'old'), true)
  assert.deepEqual(drafts, { c1: 'old' })
  drafts.c1 = 'typed meanwhile'
  assert.equal(restoreDraft(drafts, 'c1', 'old'), false)
  assert.deepEqual(drafts, { c1: 'typed meanwhile' })
})
