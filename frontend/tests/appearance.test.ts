import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  appearanceOptions,
  compileAppearance,
  defaultAppearance,
  interfaceFonts,
  messageFonts,
  sanitizeAppearance,
} from '../shared/appearance'
import { fontPackages, noticePath, renderFontLicenses } from '../scripts/build-font-licenses.mjs'
import { schemes, schemeFor, schemeTokens, themeTokens } from '../shared/schemes'
import { contrast } from '../shared/utils/color'

test('appearance sanitizing keeps valid values and replaces everything else', () => {
  assert.deepEqual(sanitizeAppearance(undefined), defaultAppearance)
  const value = sanitizeAppearance({
    mode: 'light',
    schemeDark: 'tokyo-night',
    schemeLight: 'tokyo-night', // a dark scheme in the light slot
    accent: 'custom',
    accentCustom: '#FF0000',
    messageSize: 99,
    groupSpacing: 3.5,
    messageLayout: 'irc',
    font: '"; } body { display: none',
    extra: 'dropped',
  })
  assert.equal(value.mode, 'light')
  assert.equal(value.schemeDark, 'tokyo-night')
  assert.equal(value.schemeLight, defaultAppearance.schemeLight)
  assert.equal(value.accentCustom, '#ff0000')
  assert.equal(value.messageSize, 20)
  assert.equal(value.groupSpacing, defaultAppearance.groupSpacing)
  assert.equal(value.messageLayout, 'irc')
  assert.equal(value.font, defaultAppearance.font)
  assert.equal('extra' in value, false)
})

test('every scheme yields every token, with readable text and accents', () => {
  for (const scheme of schemes) {
    const tokens = schemeTokens(scheme)
    for (const name of themeTokens)
      assert.match(tokens[name], /^#[\da-f]{6}$/i, `${scheme.id} ${name}`)
    assert.ok(contrast(tokens.text, tokens.main) >= 4.5, `${scheme.id} text`)
    assert.ok(contrast(tokens['accent-ink'], tokens.accent) >= 3, `${scheme.id} accent ink`)
    for (const name of themeTokens.filter((name) => name.startsWith('code-')))
      assert.ok(contrast(tokens[name], tokens.rail) >= 4.5, `${scheme.id} ${name}`)
  }
})

test('custom accents are adjusted to stay visible on the scheme', () => {
  const light = schemeFor('github-light', 'light')
  const tokens = schemeTokens(light, '#ffff00')
  assert.ok(contrast(tokens.accent, tokens.main) >= 3.2)
})

test('compiled appearance exposes only known attributes and declarations', () => {
  const { css, attrs } = compileAppearance(
    sanitizeAppearance({ messageLayout: 'compact', corners: 'square' }),
    'dark',
  )
  assert.deepEqual(Object.keys(attrs).sort(), [
    'data-corners',
    'data-density',
    'data-layout',
    'data-theme',
  ])
  assert.equal(attrs['data-layout'], 'compact')
  assert.match(css, /^:root:root \{ .*--radius: 2px.*; \}$/)
  assert.doesNotMatch(css, /[<>]/)
})

test('both font selectors offer eight distinct, stable choices', () => {
  assert.deepEqual([...appearanceOptions.font].sort(), [
    'atkinson',
    'geist',
    'ibm-plex',
    'inter',
    'manrope',
    'nunito',
    'source-sans',
    'system',
  ])
  assert.deepEqual([...appearanceOptions.messageFont].sort(), [
    'atkinson',
    'atkinson-mono',
    'geist-mono',
    'interface',
    'jetbrains-mono',
    'lexend',
    'literata',
    'serif',
  ])
  for (const fonts of [interfaceFonts, messageFonts]) {
    const entries = Object.values(fonts)
    assert.equal(new Set(entries.map((font) => font.label)).size, entries.length)
    assert.equal(new Set(entries.map((font) => font.stack)).size, entries.length)
  }
  assert.equal(defaultAppearance.font, 'geist')
  assert.equal(defaultAppearance.messageFont, 'interface')
})

test('new fonts survive sanitizing and compile to their own stacks', () => {
  for (const [font, family] of [['source-sans', 'Source Sans 3 Variable']] as const) {
    const value = sanitizeAppearance({ font })
    assert.equal(value.font, font)
    assert.ok(compileAppearance(value, 'dark').css.includes(`--font-ui: '${family}'`))
  }
  for (const [messageFont, family] of [
    ['lexend', 'Lexend Variable'],
    ['literata', 'Literata Variable'],
    ['atkinson-mono', 'Atkinson Hyperlegible Mono Variable'],
  ] as const) {
    const value = sanitizeAppearance({ messageFont, font: 'source-sans' })
    assert.equal(value.messageFont, messageFont)
    const { css } = compileAppearance(value, 'light')
    assert.ok(css.includes(`--font-message: '${family}'`))
    assert.ok(
      css.includes("--font-ui: 'Source Sans 3 Variable'"),
      'interface choice is independent',
    )
  }
  const inherit = compileAppearance(sanitizeAppearance({ font: 'source-sans' }), 'dark').css
  assert.ok(inherit.includes('--font-message: var(--font-ui)'), 'Interface follows the UI font')
})

test('every self-hosted font is loaded and its license notice is current', () => {
  const fontsCss = readFileSync(new URL('../app/assets/css/fonts.css', import.meta.url), 'utf8')
  const notice = readFileSync(noticePath, 'utf8')
  const packages = fontPackages()
  assert.equal(packages.length, 13)
  for (const name of packages) {
    assert.ok(fontsCss.includes(`@import '${name}';`), `${name} is imported`)
    assert.match(notice, new RegExp(`^${name} \\S+ \\(OFL-1\\.1\\)$`, 'm'), `${name} license`)
  }
  // A stale notice (a font added, bumped or relicensed) fails: run scripts/build-font-licenses.mjs.
  assert.equal(notice, renderFontLicenses())
  // Fonts load from the bundled packages only, never from a third-party host.
  assert.doesNotMatch(fontsCss, /https?:/)
})
