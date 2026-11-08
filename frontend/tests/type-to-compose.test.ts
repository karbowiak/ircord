import { test } from 'node:test'
import assert from 'node:assert/strict'
import { startsTyping, type TypedKey } from '../shared/utils/type-to-compose'

const key = (value: string, extra: Partial<TypedKey> & { altGraph?: boolean } = {}): TypedKey => ({
  key: value,
  ctrlKey: false,
  metaKey: false,
  altKey: false,
  isComposing: false,
  getModifierState: (name) => name === 'AltGraph' && !!extra.altGraph,
  ...extra,
})

test('plain characters, including astral and emoji ones, start typing', () => {
  for (const value of ['a', 'Z', '1', ' ', '@', 'é', 'ß', '中', '𝔸', '😀', '👍🏽'])
    assert.equal(startsTyping(key(value)), true, value)
})

test('named keys, shortcuts and composition updates do not', () => {
  for (const value of ['Enter', 'Escape', 'Tab', 'ArrowUp', 'F5', 'Shift', 'Unidentified', '?'])
    assert.equal(startsTyping(key(value)), false, value)
  assert.equal(startsTyping(key('k', { ctrlKey: true })), false)
  assert.equal(startsTyping(key('g', { metaKey: true })), false)
  assert.equal(startsTyping(key('a', { isComposing: true })), false)
})

test('AltGr and macOS Option characters count, Alt shortcuts elsewhere do not', () => {
  // Windows reports AltGr as Ctrl+Alt with the AltGraph modifier state.
  assert.equal(startsTyping(key('@', { ctrlKey: true, altKey: true, altGraph: true })), true)
  assert.equal(startsTyping(key('€', { ctrlKey: true, altKey: true, altGraph: true })), true)
  assert.equal(startsTyping(key('a', { ctrlKey: true, altKey: true })), false)
  assert.equal(startsTyping(key('f', { altKey: true })), false)
  assert.equal(startsTyping(key('@', { altKey: true }), true), true)
  assert.equal(startsTyping(key('a', { altKey: true, metaKey: true }), true), false)
})

test('dead keys and IME process keys start a composition in the composer', () => {
  assert.equal(startsTyping(key('Dead')), true)
  assert.equal(startsTyping(key('Process')), true)
})
