import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { brandGlyph } from '../app/utils/brand'

const favicon = readFileSync(new URL('../public/favicon.svg', import.meta.url), 'utf8')

test('the monochrome brand glyph is the favicon hash, centred in a square box', () => {
  const glyph = brandGlyph(favicon)
  assert.equal(glyph.path, /\sd="([^"]+)"/.exec(favicon)?.[1])
  assert.equal(glyph.strokeWidth, 3)
  const [left, top, width, height] = glyph.viewBox.split(' ').map(Number)
  assert.equal(width, height)
  // The hash spans x 9..30 and y 11..29, plus half the stroke on each side.
  assert.ok(Math.abs(left! + width! / 2 - 19.5) < 0.01)
  assert.ok(Math.abs(top! + height! / 2 - 20) < 0.01)
  assert.ok(Math.abs(width! - 24 * 1.5) < 0.01)
})

test('relative and absolute line commands are both followed', () => {
  const glyph = brandGlyph('<svg><path stroke-width="2" d="M0 0h10v10L0 10z"/></svg>')
  assert.equal(glyph.viewBox, '-4 -4 18 18')
})

const manifest = JSON.parse(
  readFileSync(new URL('../public/manifest.webmanifest', import.meta.url), 'utf8'),
)
const worker = readFileSync(new URL('../public/sw.js', import.meta.url), 'utf8')
const config = readFileSync(new URL('../nuxt.config.ts', import.meta.url), 'utf8')

test('the manifest, page head and service worker point at icon files that exist', () => {
  const referenced = [
    ...manifest.icons.map((icon: { src: string }) => icon.src),
    ...[...worker.matchAll(/(?:icon|badge): '([^']+)'/g)].map((m) => m[1]),
    ...[...config.matchAll(/href: '(\/[^']+\.(?:svg|png))'/g)].map((m) => m[1]),
  ]
  assert.ok(referenced.includes('/favicon.svg'))
  assert.ok(referenced.includes('/icons/badge-96.png'))
  assert.ok(referenced.includes('/icons/apple-touch-icon.png'))
  for (const src of referenced) readFileSync(new URL(`../public${src}`, import.meta.url))
  assert.ok(manifest.icons.some((icon: { purpose: string }) => icon.purpose === 'maskable'))
})
