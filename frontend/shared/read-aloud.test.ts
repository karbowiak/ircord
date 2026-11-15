import assert from 'node:assert/strict'
import test from 'node:test'
import { announcesEnabled, applyUserEnable } from './read-aloud'

test('read-aloud is confirmed aloud only when turned on by hand', () => {
  assert.equal(announcesEnabled(false, true), true)
  // Turning it off, or switching on what is already on (a page load restoring it), is silent.
  assert.equal(announcesEnabled(true, false), false)
  assert.equal(announcesEnabled(false, false), false)
  assert.equal(announcesEnabled(true, true), false)
})

test('the UI entry stores the choice and announces only when it turns read-aloud on', () => {
  const run = (was: boolean, enable: boolean) => {
    const calls: string[] = []
    applyUserEnable(
      was,
      enable,
      (value) => calls.push(`set:${value}`),
      () => calls.push('announce'),
    )
    return calls
  }
  assert.deepEqual(run(false, true), ['set:true', 'announce'])
  assert.deepEqual(run(true, false), ['set:false'])
  assert.deepEqual(run(true, true), ['set:true'])
  assert.deepEqual(run(false, false), ['set:false'])
})
