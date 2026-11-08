import { test } from 'node:test'
import assert from 'node:assert/strict'
import { isTypeaheadKey, listboxKey, placePopup, typeahead } from '../app/utils/listbox'

const options = [
  { label: 'Catppuccin Mocha' },
  { label: 'Charcoal' },
  { label: 'Disabled', disabled: true },
  { label: 'Dracula' },
  { label: 'Tokyo Night' },
]
const closed = (selected: number) => ({ open: false, active: -1, selected })
const at = (active: number, selected = 0) => ({ open: true, active, selected })

test('closed: arrows, Enter and Space open on the current choice; Home/End open at the ends', () => {
  for (const key of ['ArrowDown', 'ArrowUp', 'Enter', ' '])
    assert.deepEqual(listboxKey(key, closed(3), options), { type: 'open', active: 3 })
  assert.deepEqual(listboxKey('ArrowDown', closed(-1), options), { type: 'open', active: 0 })
  assert.deepEqual(listboxKey('Home', closed(3), options), { type: 'open', active: 0 })
  assert.deepEqual(listboxKey('End', closed(0), options), { type: 'open', active: 4 })
  assert.deepEqual(listboxKey('Escape', closed(0), options), { type: 'none' })
})

test('open: arrows skip disabled options and stop at the ends', () => {
  assert.deepEqual(listboxKey('ArrowDown', at(1), options), { type: 'move', active: 3 })
  assert.deepEqual(listboxKey('ArrowUp', at(3), options), { type: 'move', active: 1 })
  assert.deepEqual(listboxKey('ArrowDown', at(4), options), { type: 'move', active: 4 })
  assert.deepEqual(listboxKey('ArrowUp', at(0), options), { type: 'move', active: 0 })
  assert.deepEqual(listboxKey('PageDown', at(0), options), { type: 'move', active: 4 })
  assert.deepEqual(listboxKey('Home', at(4), options), { type: 'move', active: 0 })
  assert.deepEqual(listboxKey('End', at(0), options), { type: 'move', active: 4 })
})

test('open: Enter/Space choose and keep focus, Escape closes, Tab chooses and moves on', () => {
  const select = (index: number, keepFocus: boolean) => ({
    type: 'select',
    index,
    close: true,
    keepFocus,
  })
  assert.deepEqual(listboxKey('Enter', at(3), options), select(3, true))
  assert.deepEqual(listboxKey(' ', at(3), options), select(3, true))
  assert.deepEqual(listboxKey('ArrowUp', { ...at(4), altKey: true }, options), select(4, true))
  assert.deepEqual(listboxKey('Escape', at(3), options), { type: 'close' })
  assert.deepEqual(listboxKey('Tab', at(3), options), select(3, false))
  // Tab on the current value is not a change.
  assert.deepEqual(listboxKey('Tab', at(0, 0), options), { type: 'close' })
  assert.deepEqual(listboxKey('Enter', at(2), options), { type: 'close' })
})

test('type-ahead matches prefixes, cycles on a repeated letter and skips disabled options', () => {
  assert.equal(typeahead(options, 't', 0), 4)
  assert.equal(typeahead(options, 'ch', 0), 1)
  assert.equal(typeahead(options, 'c', 0), 1)
  assert.equal(typeahead(options, 'cc', 1), 0, 'a repeated letter wraps to the next match')
  assert.equal(typeahead(options, 'd', 0), 3, 'the disabled option is skipped')
  assert.equal(typeahead(options, 'CATP', -1), 0, 'any case')
  assert.equal(typeahead(options, 'x', 0), -1)
  assert.ok(isTypeaheadKey({ key: 'a', ctrlKey: false, metaKey: false, altKey: false }))
  assert.ok(!isTypeaheadKey({ key: 'a', ctrlKey: true, metaKey: false, altKey: false }))
  assert.ok(!isTypeaheadKey({ key: 'ArrowDown', ctrlKey: false, metaKey: false, altKey: false }))
})

test('the popup opens below, flips above near the bottom and stays inside the viewport', () => {
  const viewport = { width: 400, height: 800 }
  const below = placePopup({ top: 100, bottom: 136, left: 20, width: 220 }, viewport, 300)
  assert.equal(below.up, false)
  assert.equal(below.top, 140)
  assert.equal(below.maxHeight, 300)
  const above = placePopup({ top: 740, bottom: 776, left: 300, width: 220 }, viewport, 300)
  assert.equal(above.up, true)
  assert.equal(above.bottom, 800 - 740 + 4)
  assert.equal(above.left, 400 - 8 - 220, 'pulled back inside the right edge')
})
