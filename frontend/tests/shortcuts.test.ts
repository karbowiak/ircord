import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  ariaShortcut,
  chordLabel,
  isMac,
  matchesChord,
  shortcutLabel,
  shortcutOf,
  type KeyLike,
} from '../shared/utils/shortcuts'

const press = (key: string, extra: Partial<KeyLike> = {}): KeyLike => ({
  key,
  code: key.length === 1 && /[a-z]/i.test(key) ? `Key${key.toUpperCase()}` : '',
  ctrlKey: false,
  metaKey: false,
  shiftKey: false,
  altKey: false,
  ...extra,
})

test('Ctrl and ⌘ both trigger mod shortcuts', () => {
  assert.equal(shortcutOf(press('g', { ctrlKey: true }), ['gifs', 'search']), 'gifs')
  assert.equal(shortcutOf(press('g', { metaKey: true }), ['gifs', 'search']), 'gifs')
  assert.equal(shortcutOf(press('k', { metaKey: true }), ['gifs', 'search']), 'search')
  // Caps lock reports an upper-case key.
  assert.equal(shortcutOf(press('K', { ctrlKey: true }), ['search']), 'search')
  assert.equal(shortcutOf(press('g'), ['gifs']), undefined)
})

test('extra modifiers do not trigger a shortcut', () => {
  assert.equal(shortcutOf(press('g', { ctrlKey: true, shiftKey: true }), ['gifs']), undefined)
  assert.equal(shortcutOf(press('g', { ctrlKey: true, metaKey: true }), ['gifs']), undefined)
  // Ctrl Alt is AltGr on Windows: it types characters, so it never matches Ctrl shortcuts.
  assert.equal(shortcutOf(press('k', { ctrlKey: true, altKey: true }), ['search']), undefined)
  assert.equal(shortcutOf(press('?', { ctrlKey: true, shiftKey: true }), ['help']), undefined)
  assert.equal(shortcutOf(press('ArrowDown', { altKey: true }), ['nextUnread']), undefined)
  assert.equal(shortcutOf(press('Escape', { shiftKey: true }), ['markRead']), undefined)
})

test('punctuation shortcuts ignore the Shift a layout needs to type them', () => {
  assert.equal(shortcutOf(press('?', { shiftKey: true }), ['help']), 'help')
  assert.equal(shortcutOf(press('?'), ['help']), 'help')
  // German layouts type “/” with Shift 7.
  assert.equal(shortcutOf(press('/', { ctrlKey: true, shiftKey: true }), ['help']), 'help')
  assert.equal(shortcutOf(press(',', { metaKey: true }), ['settings']), 'settings')
})

test('non-Latin layouts use the key position for letters only', () => {
  assert.ok(matchesChord(press('п', { code: 'KeyG', ctrlKey: true }), { key: 'g', mod: true }))
  // A punctuation key in a letter position (AZERTY “,” on KeyM) is still “,”.
  assert.equal(shortcutOf(press(',', { code: 'KeyM', ctrlKey: true }), ['settings']), 'settings')
  assert.equal(shortcutOf(press(',', { code: 'KeyM', ctrlKey: true }), ['search']), undefined)
})

test('labels follow the platform', () => {
  assert.equal(shortcutLabel('gifs', false), 'Ctrl G')
  assert.equal(shortcutLabel('gifs', true), '⌘ G')
  assert.equal(shortcutLabel('help', false), '? · Ctrl /')
  assert.equal(chordLabel({ key: 'ArrowDown', alt: true, shift: true }, false), 'Alt Shift ↓')
  assert.equal(chordLabel({ key: 'ArrowDown', alt: true, shift: true }, true), '⌥ ⇧ ↓')
  assert.equal(ariaShortcut('gifs', false), 'Control+G')
  assert.equal(ariaShortcut('search', true), 'Meta+K')
  assert.ok(isMac('MacIntel') && isMac('iPhone') && !isMac('Linux x86_64') && !isMac('Win32'))
})
