import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  SEED_SCORES,
  TABLE_SIZE,
  NAME_CHARACTERS,
  NAME_LENGTH,
  clipName,
  insertScore,
  placesIn,
  tableName,
  validScore,
} from '../shared/utils/hash-field-scores'

const full = Array.from({ length: TABLE_SIZE }, (_, i) => ({
  rank: i + 1,
  name: `p${i}`,
  score: (TABLE_SIZE - i) * 100,
}))

test('a new table starts with the two default entries, best first', () => {
  assert.deepEqual(SEED_SCORES, [
    { name: 'Michael Karbowiak', score: 97909 },
    { name: 'Peter Petermann', score: 22831 },
  ])
})

test('a score places while the table has room, or when it beats the lowest entry', () => {
  assert.equal(placesIn(full.slice(0, 3), 10), true)
  assert.equal(placesIn(full, 100), false, 'a tie keeps the earlier entry')
  assert.equal(placesIn(full, 110), true)
  assert.equal(placesIn([], 0), false, 'a game without a hit never places')
})

test('only plausible scores are valid', () => {
  for (const score of [10, 520, 10_000_000]) assert.equal(validScore(score), true, String(score))
  for (const score of [0, -10, 15, 10_000_010, 1.5, Number.NaN])
    assert.equal(validScore(score), false, String(score))
})

test('table names are trimmed, folded and bounded', () => {
  assert.deepEqual(tableName('  Ada \t Lovelace '), { name: 'Ada Lovelace' })
  assert.deepEqual(tableName('   '), { error: NAME_LENGTH })
  assert.deepEqual(tableName('x'.repeat(25)), { error: NAME_LENGTH })
  assert.deepEqual(tableName('👾'.repeat(24)), { name: '👾'.repeat(24) })
  assert.deepEqual(tableName('👨‍👩‍👧 Zoë'), { name: '👨‍👩‍👧 Zoë' })
})

test('names with characters the table cannot show are refused for that reason', () => {
  for (const raw of [
    'bell\u0007',
    'right\u202eleft',
    'private\ue000',
    'non\uffff',
    'half\ud83d',
    'bad\ufffd',
  ])
    assert.deepEqual(tableName(raw), { error: NAME_CHARACTERS }, JSON.stringify(raw))
})

test('a long default name is clipped by character, never splitting an emoji', () => {
  const name = 'a'.repeat(23) + '😀' + 'b'
  assert.equal(clipName(name), 'a'.repeat(23) + '😀')
  assert.deepEqual(tableName(clipName(name)), { name: 'a'.repeat(23) + '😀' })
})

test('inserting keeps the top entries and ranks a tie after the earlier score', () => {
  const tie = insertScore(full, 'late', 500)
  assert.equal(tie.rank, 7)
  assert.equal(tie.items[5]!.name, 'p5')
  assert.equal(tie.items.length, TABLE_SIZE)
  assert.equal(tie.items.at(-1)!.name, 'p8')
  assert.deepEqual(insertScore(full, 'low', 50), { rank: 0, items: full })
  assert.equal(insertScore([], 'first', 10).rank, 1)
})
