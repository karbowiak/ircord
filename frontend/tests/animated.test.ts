import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isAnimatedImage } from '../shared/utils/animated'

const bytes = (...parts: (string | number[])[]) =>
  new Uint8Array(
    parts.flatMap((p) => (typeof p === 'string' ? [...p].map((c) => c.charCodeAt(0)) : p)),
  )
const u32 = (n: number) => [(n >>> 24) & 255, (n >>> 16) & 255, (n >>> 8) & 255, n & 255]
const png = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]
const chunk = (type: string, length: number) => [
  ...u32(length),
  ...bytes(type),
  ...new Array(length + 4).fill(0),
]

test('animated images are told apart from still ones by their first bytes', () => {
  assert.equal(isAnimatedImage(bytes('GIF89a', [1, 0, 1, 0])), true)
  // PNG: acTL before IDAT is an APNG; IDAT first is a still PNG.
  assert.equal(
    isAnimatedImage(
      new Uint8Array([...png, ...chunk('IHDR', 13), ...chunk('acTL', 8), ...chunk('IDAT', 4)]),
    ),
    true,
  )
  assert.equal(
    isAnimatedImage(
      new Uint8Array([...png, ...chunk('IHDR', 13), ...chunk('IDAT', 4), ...chunk('acTL', 8)]),
    ),
    false,
  )
  // WebP: the VP8X animation flag (0x02).
  assert.equal(
    isAnimatedImage(bytes('RIFF', u32(0), 'WEBP', 'VP8X', u32(10), [0x12, 0, 0, 0])),
    true,
  )
  assert.equal(
    isAnimatedImage(bytes('RIFF', u32(0), 'WEBP', 'VP8X', u32(10), [0x10, 0, 0, 0])),
    false,
  )
  assert.equal(isAnimatedImage(bytes('RIFF', u32(0), 'WEBP', 'VP8 ', u32(10))), false)
  // AVIF: the avis brand marks an image sequence.
  assert.equal(
    isAnimatedImage(bytes(u32(28), 'ftyp', 'avis', u32(0), 'avif', 'avis', 'mif1')),
    true,
  )
  assert.equal(isAnimatedImage(bytes(u32(24), 'ftyp', 'avif', u32(0), 'avif', 'mif1')), false)
  assert.equal(isAnimatedImage(bytes([0xff, 0xd8, 0xff, 0xe0])), false)
  assert.equal(isAnimatedImage(new Uint8Array()), false)
})
