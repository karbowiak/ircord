import { test } from 'node:test'
import assert from 'node:assert/strict'
import { unseenSince } from '../shared/utils/unseen'

test('counts only messages after the one the reader last had', () => {
  assert.equal(unseenSince(['a', 'b', 'c'], 'c'), 0)
  assert.equal(unseenSince(['a', 'b', 'c', 'd', 'e'], 'c'), 2)
})

test('older history loaded above the view is not counted', () => {
  // Scrolled up at c, then two older pages arrived above it and one new message below.
  assert.equal(unseenSince(['x', 'y', 'z', 'a', 'b', 'c', 'd'], 'c'), 1)
  assert.equal(unseenSince(['x', 'y', 'z', 'a', 'b', 'c'], 'c'), 0)
})

test('nothing is counted without the anchor message', () => {
  assert.equal(unseenSince(['a', 'b'], undefined), 0)
  assert.equal(unseenSince(['a', 'b'], 'gone'), 0)
})
