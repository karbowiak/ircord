import assert from 'node:assert/strict'
import test from 'node:test'
import { announcesEnabled } from './read-aloud'

test('read-aloud is confirmed aloud only when turned on by hand', () => {
  assert.equal(announcesEnabled(false, true), true)
  // Turning it off, or switching on what is already on (a page load restoring it), is silent.
  assert.equal(announcesEnabled(true, false), false)
  assert.equal(announcesEnabled(false, false), false)
  assert.equal(announcesEnabled(true, true), false)
})
