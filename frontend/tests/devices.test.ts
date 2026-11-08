import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  deviceName,
  mobileDevice,
  notificationText,
  presence,
  relativeTime,
} from '../shared/utils/devices'

const now = Date.parse('2026-11-08T12:00:00Z')
const ago = (ms: number) => new Date(now - ms).toISOString()

test('device names fall back honestly when the browser did not say', () => {
  assert.equal(deviceName({ browser: 'Firefox', os: 'Linux' }), 'Firefox on Linux')
  assert.equal(deviceName({ browser: '', os: 'Android' }), 'Android')
  assert.equal(deviceName({ browser: 'Safari', os: '' }), 'Safari')
  assert.equal(deviceName({ browser: '', os: '' }), 'Unknown browser')
  assert.ok(mobileDevice({ os: 'iOS' }))
  assert.ok(!mobileDevice({ os: 'Windows' }))
})

test('relative times are readable and coarse', () => {
  assert.equal(relativeTime(ago(10_000), now), 'just now')
  assert.equal(relativeTime(ago(5 * 60_000), now), '5 minutes ago')
  assert.equal(relativeTime(ago(3 * 3_600_000), now), '3 hours ago')
  assert.equal(relativeTime(ago(2 * 86_400_000), now), '2 days ago')
})

test('presence uses fresh evidence, not the existence of a session', () => {
  const base = { current: false, lastSeenAt: ago(2 * 3_600_000) }
  assert.equal(presence({ ...base, status: 'connected' }, now), 'Connected now')
  assert.equal(presence({ ...base, status: 'offline' }, now), 'Last seen 2 hours ago')
  assert.equal(
    presence({ current: false, status: 'unknown', lastSeenAt: undefined }, now),
    'Last seen: unknown',
  )
  assert.equal(presence({ ...base, current: true, status: 'recent' }, now), 'Active now')
})

test('notification status separates registered push from this device’s own mode', () => {
  assert.equal(notificationText({ current: false, push: 'registered' }), 'Push registered')
  assert.equal(notificationText({ current: false, push: 'none' }), 'No push')
  assert.equal(notificationText({ current: true, push: 'none' }, 'off'), 'Notifications: off')
  assert.equal(
    notificationText({ current: true, push: 'none' }, 'open'),
    'Notifications: while open',
  )
  assert.equal(
    notificationText({ current: true, push: 'registered' }, 'push'),
    'Notifications: push',
  )
  assert.equal(
    notificationText({ current: true, push: 'none' }, 'push'),
    'Notifications: push (not registered)',
  )
})
