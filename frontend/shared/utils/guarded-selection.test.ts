import assert from 'node:assert/strict'
import test from 'node:test'
import { guardedSelectionResult } from './guarded-selection'

test('conversation results seed only the original account and activate only the original selection', () => {
  const initial = { account: 'a', network: 'n', revision: 3 }
  assert.deepEqual(guardedSelectionResult(initial, initial, 'n'), { seed: true, activate: true })
  assert.deepEqual(guardedSelectionResult(initial, { ...initial, revision: 4 }, 'n'), {
    seed: true,
    activate: false,
  })
  assert.deepEqual(guardedSelectionResult(initial, { ...initial, network: 'other' }, 'n'), {
    seed: true,
    activate: false,
  })
  assert.deepEqual(guardedSelectionResult(initial, { ...initial, account: 'other' }, 'n'), {
    seed: false,
    activate: false,
  })
  assert.deepEqual(guardedSelectionResult(initial, initial, 'other'), {
    seed: false,
    activate: false,
  })
})
