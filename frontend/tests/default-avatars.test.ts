import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import {
  DEFAULT_AVATAR_PATH,
  defaultAvatarFor,
  defaultAvatarUrl,
  avatarIndex,
  fnv1a,
  parseAvatarManifest,
  suggestAvatar,
} from '../shared/utils/default-avatars'

const dir = join(import.meta.dirname, '../public', DEFAULT_AVATAR_PATH)
const raw = JSON.parse(readFileSync(join(dir, 'manifest.json'), 'utf8'))
const shipped = parseAvatarManifest(raw)

test('FNV-1a matches the published 32-bit test vectors', () => {
  assert.equal(fnv1a(''), 0x811c9dc5)
  assert.equal(fnv1a('a'), 0xe40c292c)
  assert.equal(fnv1a('foobar'), 0xbf9cf968)
  // UTF-8 bytes, not UTF-16 code units.
  assert.equal(fnv1a('ü'), fnv1a('ü'))
  assert.notEqual(fnv1a('ü'), fnv1a('ü'.normalize('NFD')))
})

test('the suggestion for a username is stable and ignores case and surrounding spaces', () => {
  // Pinned: changing the hash or the manifest order would change everyone's suggestion.
  // Values checked against an independent FNV-1a + murmur3-fmix32 implementation.
  const pinned = Object.fromEntries(
    ['karbowiak', 'peterpowers', 'mika', 'alice'].map((name) => [
      name,
      suggestAvatar(name, shipped)?.id,
    ]),
  )
  assert.deepEqual(pinned, {
    karbowiak: 'avatar-032',
    peterpowers: 'avatar-020',
    mika: 'avatar-004',
    alice: 'avatar-031',
  })
  // The raw names hash apart, so only the lowercasing makes these agree.
  assert.notEqual(avatarIndex('KarBowiak', 32), avatarIndex('karbowiak', 32))
  assert.notEqual(avatarIndex('ALICE', 32), avatarIndex('alice', 32))
  assert.equal(suggestAvatar('  KarBowiak ', shipped), suggestAvatar('karbowiak', shipped))
  assert.equal(suggestAvatar('ALICE', shipped), suggestAvatar('alice', shipped))
})

test('every bit of every character counts, not only what survives the modulo', () => {
  // Bytes equal mod 32 would always collide with FNV-1a alone for a 32-entry set.
  assert.notEqual(avatarIndex('user1', 32), avatarIndex('userq', 32))
  assert.notEqual(avatarIndex('p0', 32), avatarIndex('pp', 32))
  const counts = new Array(32).fill(0)
  for (let i = 0; i < 3200; i++) counts[avatarIndex(`user${i}`, 32)]++
  assert.ok(Math.min(...counts) > 50 && Math.max(...counts) < 150, `spread ${counts}`)
})

test('empty names and empty sets have no suggestion', () => {
  assert.equal(suggestAvatar('', shipped), undefined)
  assert.equal(suggestAvatar('   ', shipped), undefined)
  assert.equal(suggestAvatar('alice', []), undefined)
})

test('the shipped set is the 32 starter avatars, each file present, with no generation details', () => {
  assert.equal(shipped.length, 32)
  const count = (category: string) => shipped.filter((a) => a.category === category).length
  assert.deepEqual([count('male'), count('female'), count('furry')], [15, 15, 2])
  for (const avatar of shipped) assert.ok(existsSync(join(dir, avatar.file)), avatar.file)
  for (const entry of raw.avatars)
    assert.deepEqual(Object.keys(entry).sort(), ['category', 'file', 'id'])
})

test('new manifest entries become selectable and suggestable without code changes', () => {
  const grown = parseAvatarManifest({
    ...raw,
    avatars: [...raw.avatars, { id: 'avatar-033', file: 'avatar-033.png', category: 'robot' }],
  })
  assert.equal(grown.length, 33)
  assert.deepEqual(grown.at(-1), { id: 'avatar-033', file: 'avatar-033.png', category: 'robot' })
  const name = Array.from({ length: 500 }, (_, i) => `user${i}`).find(
    (n) => suggestAvatar(n, grown)?.id === 'avatar-033',
  )
  assert.ok(name, 'some username is suggested the new avatar')
})

test('malformed, unsafe and repeated manifest entries are skipped', () => {
  assert.deepEqual(parseAvatarManifest(null), [])
  assert.deepEqual(parseAvatarManifest({ avatars: 'nope' }), [])
  const parsed = parseAvatarManifest({
    avatars: [
      { id: 'ok', file: 'ok.png', category: 'male' },
      { id: 'ok', file: 'again.png' },
      { id: 'up', file: '../secret.png' },
      { id: 'abs', file: 'https://elsewhere.example/x.png' },
      { id: 'svg', file: 'x.svg' },
      { file: 'noid.png' },
      { id: 'nocat', file: 'nocat.webp' },
    ],
  })
  assert.deepEqual(parsed, [
    { id: 'ok', file: 'ok.png', category: 'male' },
    { id: 'nocat', file: 'nocat.webp', category: '' },
  ])
})

test('a default is saved as an HTTPS URL on this origin and recognised again', () => {
  const origin = 'https://ircord.example'
  const avatar = shipped[4]!
  const url = defaultAvatarUrl(avatar, origin)
  assert.equal(url, 'https://ircord.example/avatars/defaults/avatar-005.png')
  assert.equal(defaultAvatarFor(url, shipped, origin), avatar)
  assert.equal(defaultAvatarFor(url, shipped, 'https://other.example'), undefined)
  assert.equal(
    defaultAvatarFor('https://ircord.example/files/abc.webp', shipped, origin),
    undefined,
  )
  assert.equal(
    defaultAvatarFor('https://ircord.example/avatars/defaults/gone.png', shipped, origin),
    undefined,
  )
  // Same length as the defaults path, so only the prefix check rejects it.
  assert.equal(
    defaultAvatarFor('https://ircord.example/xxxxxxxxxxxxxxxxxavatar-005.png', shipped, origin),
    undefined,
  )
  assert.equal(defaultAvatarFor(undefined, shipped, origin), undefined)
  assert.equal(defaultAvatarFor('not a url', shipped, 'not an origin'), undefined)
})
