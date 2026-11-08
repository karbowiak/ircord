import { test } from 'node:test'
import assert from 'node:assert/strict'
import { unseenSince } from '../shared/utils/unseen'

type Item = { id: string; createdAt: string; deleted?: boolean; kind?: string }
const item = (id: string, minute: number, extra: Partial<Item> = {}): Item => ({
  id,
  createdAt: new Date(Date.UTC(2026, 10, 8, 12, minute)).toISOString(),
  ...extra,
})
const list = (...ids: string[]) => ids.map((id, i) => item(id, i + 10))
const all = () => true
const seen = (items: Item[], id: string) => ({
  id,
  createdAt: items.find((m) => m.id === id)!.createdAt,
})

test('counts only messages after the one the reader last had', () => {
  const items = list('a', 'b', 'c', 'd', 'e')
  assert.equal(unseenSince(items, seen(items, 'e'), all), 0)
  assert.equal(unseenSince(items, seen(items, 'c'), all), 2)
})

test('older history loaded above the view is not counted', () => {
  // Scrolled up at c, then older pages arrived above it and one new message below.
  const items = [item('x', 1), item('y', 2), ...list('a', 'b', 'c', 'd')]
  assert.equal(unseenSince(items, seen(items, 'c'), all), 1)
})

test('a deleted anchor still marks where new messages start', () => {
  const items = [...list('a', 'b'), item('c', 30, { deleted: true }), item('d', 31), item('e', 32)]
  assert.equal(
    unseenSince(items, seen(items, 'c'), (m) => !m.deleted),
    2,
  )
})

test('rows that are not messages are not counted', () => {
  const items = [...list('a'), item('j', 20, { kind: 'event' }), item('m', 21, { kind: 'message' })]
  assert.equal(
    unseenSince(items, seen(items, 'a'), (m) => m.kind === 'message'),
    1,
  )
})

test('an anchor trimmed from the window counts what is newer than it', () => {
  const anchor = { id: 'gone', createdAt: item('gone', 15).createdAt }
  assert.equal(unseenSince([item('a', 1), item('b', 16), item('c', 17)], anchor, all), 2)
})

test('nothing is counted before the reader scrolls up', () => {
  assert.equal(unseenSince(list('a', 'b'), undefined, all), 0)
})
