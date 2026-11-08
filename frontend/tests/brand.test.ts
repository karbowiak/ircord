import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { brandGlyph } from '../app/utils/brand'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')
const favicon = read('../public/favicon.svg')
const glyphOf = (d: string, strokeWidth = 2) =>
  brandGlyph(`<svg><path stroke-width="${strokeWidth}" d="${d}"/></svg>`)

test('the monochrome brand glyph is the favicon hash, centred in a square box', () => {
  const glyph = brandGlyph(favicon)
  assert.equal(glyph.path, /\sd="([^"]+)"/.exec(favicon)?.[1])
  assert.equal(glyph.strokeWidth, 3)
  const [left, top, width, height] = glyph.viewBox.split(' ').map(Number)
  assert.equal(width, height)
  // The hash spans x 9..30 and y 11..29 plus half the stroke, and fills 20 of 24 like Lucide.
  assert.ok(Math.abs(left! + width! / 2 - 19.5) < 0.01)
  assert.ok(Math.abs(top! + height! / 2 - 20) < 0.01)
  assert.ok(Math.abs(width! - 24 / (5 / 6)) < 0.01)
})

test('relative and absolute line commands are both followed', () => {
  // Read as absolute, h10/v10 would land at x 10 and y 10 and give "0.3 2.8 14.4 14.4".
  assert.equal(glyphOf('M5 5h10v10L5 15').viewBox, '2.8 2.8 14.4 14.4')
})

test('closepath returns to the subpath start and exponents are numbers', () => {
  assert.equal(glyphOf('M10 10h10zm0 20h10').viewBox, '1.8 6.8 26.4 26.4')
  assert.equal(glyphOf('M0 0L1e1 0').viewBox, '-2.2 -7.2 14.4 14.4')
})

test('artwork the glyph cannot follow fails loudly instead of drawing a wrong icon', () => {
  assert.throws(() => glyphOf('M0 0a5 5 0 0 1 10 0'), /unsupported command a/)
  assert.throws(() => glyphOf('M0 0c0 -20 10 -20 10 0'), /unsupported command c/)
  assert.throws(() => brandGlyph('<svg><rect width="4" height="4"/></svg>'), /no path/)
})

test('the stroke width comes from the path or its group, not other shapes', () => {
  const svg =
    '<svg><rect stroke-width="1" width="4" height="4"/><g stroke-width="3"><path d="M0 0h10"/></g></svg>'
  assert.equal(brandGlyph(svg).strokeWidth, 3)
})

const manifest = JSON.parse(read('../public/manifest.webmanifest'))

test('every icon the manifest, page head, service worker and notifications use exists', () => {
  const referenced = [
    ...manifest.icons.map((icon: { src: string }) => icon.src),
    ...[...read('../public/sw.js').matchAll(/(?:icon|badge): '([^']+)'/g)].map((m) => m[1]),
    ...[...read('../nuxt.config.ts').matchAll(/href: '(\/[^']+\.(?:svg|png))'/g)].map((m) => m[1]),
    ...[...read('../app/composables/useNotifications.ts').matchAll(/'(\/icons\/[^']+)'/g)].map(
      (m) => m[1],
    ),
  ]
  for (const src of [
    '/favicon.svg',
    '/icons/badge-96.png',
    '/icons/apple-touch-icon.png',
    '/icons/icon-192.png',
  ])
    assert.ok(referenced.includes(src), src)
  for (const src of referenced) readFileSync(new URL(`../public${src}`, import.meta.url))
  assert.ok(manifest.icons.some((icon: { purpose: string }) => icon.purpose === 'maskable'))
})
