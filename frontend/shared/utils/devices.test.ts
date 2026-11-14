import assert from 'node:assert/strict'
import test from 'node:test'
import { relativeTime } from './devices'

const now = Date.parse('2026-11-14T12:00:00Z')
const before = (days: number) => new Date(now - days * 86_400_000).toISOString()

test('relative times step from days to weeks to months', () => {
  assert.equal(relativeTime(before(3), now), '3 days ago')
  assert.equal(relativeTime(before(15), now), '2 weeks ago')
  assert.equal(relativeTime(before(45), now), '1 month ago')
})
