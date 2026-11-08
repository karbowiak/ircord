import { test } from 'node:test'
import assert from 'node:assert/strict'
import { alertText, formFactor } from '../shared/utils/activity'
import {
  defaultNotificationSettings,
  formatDuration,
  sanitizeNotificationSettings,
} from '../shared/notifications'

const iphone =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1'
const ipadDesktopMode =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15'
const androidPhone =
  'Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Mobile Safari/537.36'
const androidTablet =
  'Mozilla/5.0 (Linux; Android 15; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36'
const firefoxLinux = 'Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0'

test('form factor tells phones, tablets and desktops apart, including iPadOS desktop mode', () => {
  assert.equal(formFactor({ userAgent: iphone }), 'phone')
  assert.equal(formFactor({ userAgent: androidPhone }), 'phone')
  assert.equal(formFactor({ userAgent: androidTablet }), 'tablet')
  assert.equal(formFactor({ userAgent: ipadDesktopMode, platform: 'MacIntel', maxTouchPoints: 5 }), 'tablet')
  assert.equal(formFactor({ userAgent: ipadDesktopMode, platform: 'MacIntel', maxTouchPoints: 0 }), 'desktop')
  assert.equal(formFactor({ userAgent: firefoxLinux }), 'desktop')
  assert.equal(formFactor({ userAgent: firefoxLinux, coarse: true, shortSide: 400 }), 'phone')
})

test('a reminder leads with the unread count and carries the latest item', () => {
  const alert = {
    conversationId: 'dm',
    messageId: 'm9',
    title: 'bob in #ops',
    body: 'are you there?',
    kind: 'mention' as const,
    badge: 3,
    unread: 10,
    conversations: 3,
    reminder: true,
  }
  assert.deepEqual(alertText(alert), {
    title: '10 unread notifications in 3 conversations',
    body: 'bob in #ops: are you there?',
  })
  assert.deepEqual(alertText({ ...alert, conversations: 1 }).title, '10 unread notifications')
  assert.deepEqual(alertText({ ...alert, reminder: false, unread: 1 }), {
    title: 'bob in #ops',
    body: 'are you there?',
  })
})

test('routing delays default, stay in bounds and keep every device after the last one', () => {
  const d = sanitizeNotificationSettings({})
  assert.equal(d.lastDeviceDelay, 30)
  assert.equal(d.allDevicesDelay, 300)
  assert.equal(d.autoAwayDelay, 600)
  assert.equal(defaultNotificationSettings.allDevicesDelay - defaultNotificationSettings.lastDeviceDelay, 270)
  const s = sanitizeNotificationSettings({ lastDeviceDelay: 120, allDevicesDelay: 60, autoAwayDelay: -5 })
  assert.equal(s.lastDeviceDelay, 120)
  assert.equal(s.allDevicesDelay, 120)
  assert.equal(s.autoAwayDelay, 0)
  const big = sanitizeNotificationSettings({ lastDeviceDelay: 1e9, allDevicesDelay: 'soon' })
  assert.equal(big.lastDeviceDelay, 3600)
  assert.equal(big.allDevicesDelay, 3600)
})

test('durations read naturally', () => {
  assert.equal(formatDuration(270), '4 minutes 30 seconds')
  assert.equal(formatDuration(30), '30 seconds')
  assert.equal(formatDuration(3600), '1 hour')
  assert.equal(formatDuration(0), '0 seconds')
  assert.equal(formatDuration(105), '1 minute 45 seconds')
})
