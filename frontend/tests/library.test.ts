import { test } from 'node:test'
import assert from 'node:assert/strict'
import { emptyLibrary, recordUsage, sanitizeLibrary, savedGif, usageScore } from '../shared/library'
import type { GifItem } from '../shared/types/media'

const gif = (id: string): GifItem => ({
  id,
  slug: `slug-${id}`,
  title: `GIF ${id}`,
  url: `https://static.klipy.com/${id}.gif`,
  previewUrl: `https://static.klipy.com/${id}.webp`,
  stillUrl: `https://static.klipy.com/${id}.jpg`,
  width: 320,
  height: 240,
  provider: 'klipy',
})

test('library sanitizing drops unsafe GIFs, duplicates and invalid ids', () => {
  const library = sanitizeLibrary({
    gifs: [
      savedGif(gif('a')),
      savedGif(gif('a')),
      { ...gif('b'), url: 'https://evil.example/x.gif' },
      { ...savedGif(gif('c')), injected: true },
    ],
    emojiUsage: [
      { id: '1F600', count: 3, lastUsed: '2026-01-01T00:00:00Z' },
      { id: '1F600', count: 9, lastUsed: '2026-01-01T00:00:00Z' },
      { id: 'has space', count: 1, lastUsed: '2026-01-01T00:00:00Z' },
      { id: '1F601', count: -2, lastUsed: '2026-01-01T00:00:00Z' },
    ],
    skinTone: '9',
  })
  assert.deepEqual(
    library.gifs.map((g) => g.id),
    ['a', 'c'],
  )
  assert.equal('injected' in library.gifs[1]!, false)
  assert.deepEqual(
    library.emojiUsage.map((u) => [u.id, u.count]),
    [['1F600', 3]],
  )
  assert.equal(library.skinTone, '0')
})

test('an empty library is valid', () => {
  assert.deepEqual(sanitizeLibrary(emptyLibrary()), emptyLibrary())
  assert.deepEqual(sanitizeLibrary('garbage'), emptyLibrary())
})

test('emoji usage counts accumulate and recent use outranks stale heavy use', () => {
  const now = new Date('2026-06-01T00:00:00Z')
  const old = { id: 'old', count: 10, lastUsed: '2026-01-01T00:00:00Z' }
  let list = recordUsage([old], { fresh: 1 }, now)
  list = recordUsage(list, { fresh: 2, 'bad id': 5 }, now)
  assert.equal(list.find((u) => u.id === 'fresh')?.count, 3)
  assert.equal(
    list.some((u) => u.id === 'bad id'),
    false,
  )
  assert.equal(list[0]!.id, 'fresh')
  assert.ok(usageScore(old, now.getTime()) < 1)
})
