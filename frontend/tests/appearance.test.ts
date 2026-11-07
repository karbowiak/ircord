import { test } from 'node:test'
import assert from 'node:assert/strict'
import { compileAppearance, defaultAppearance, sanitizeAppearance } from '../shared/appearance'
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
