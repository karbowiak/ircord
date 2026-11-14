import assert from 'node:assert/strict'
import test from 'node:test'
import { isAvatarFrame, languageName, localTime, statusDeadline, statusShowing } from './profile'

test('local time follows the timezone and ignores unknown ones', () => {
  const at = new Date('2026-11-14T12:00:00Z')
  assert.equal(localTime('Europe/Copenhagen', at, 'en-GB', false), '13:00')
  assert.equal(localTime('Not/AZone', at), undefined)
  assert.equal(localTime(undefined, at), undefined)
})

test('languages are named, unknown tags kept', () => {
  assert.equal(languageName('da', 'en'), 'Danish')
  assert.equal(languageName('not a tag', 'en'), 'not a tag')
})

test('a status shows until its deadline', () => {
  const now = Date.parse('2026-11-14T12:00:00Z')
  assert.equal(statusShowing({}, now), false)
  assert.equal(statusShowing({ statusText: 'lunch' }, now), true)
  assert.equal(
    statusShowing({ statusEmoji: '🍕', statusExpiresAt: '2026-11-14T11:00:00Z' }, now),
    false,
  )
  assert.equal(
    statusShowing({ statusText: 'lunch', statusExpiresAt: '2026-11-14T13:00:00Z' }, now),
    true,
  )
})

test('status deadlines and frame ids', () => {
  const now = new Date('2026-11-14T12:00:00Z')
  assert.equal(statusDeadline('1h', now), '2026-11-14T13:00:00.000Z')
  assert.equal(statusDeadline('never', now), null)
  assert.equal(isAvatarFrame('glow'), true)
  assert.equal(isAvatarFrame('sparkles'), false)
})
