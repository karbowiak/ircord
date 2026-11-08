import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  appearanceDefaults,
  defaultAppearance,
  interfaceAppearanceKeys,
  messageAppearanceKeys,
} from '../shared/appearance'
import {
  settingEntry,
  settingsDestination,
  settingsGroups,
  settingsSections,
} from '../shared/settings-index'

const source = (file: string) =>
  readFileSync(new URL(`../app/components/${file}`, import.meta.url), 'utf8')
const access = { emoteManager: false, push: false, quietHours: false, install: false }

test('every section is listed under one of the nav groups, Messages & media among them', () => {
  for (const section of settingsSections)
    assert.ok(
      settingsGroups.some((g) => g.id === section.group),
      section.id,
    )
  assert.equal(settingsDestination('messages', access).section, 'messages')
})

test('message presentation and playback live under Messages & media, not Notifications', () => {
  for (const id of [
    'channel-activity',
    'link-previews',
    'autoplay-media',
    'animate-media',
    'ignored-people',
    'message-layout',
    'clock',
  ])
    assert.equal(settingEntry(id)?.section, 'messages', id)
  const notifications = source('NotificationSettings.vue')
  for (const label of ['Channel activity', 'Image and video previews', 'Autoplay GIFs and videos'])
    assert.ok(!notifications.includes(label), `${label} is not under Notifications`)
  for (const file of ['AppearanceSettings.vue', 'MessageSettings.vue', 'NotificationSettings.vue'])
    assert.ok(!/under Notifications/.test(source(file)), `${file} has no stale reference`)
})

test('Appearance and Messages split the appearance keys and reset only their own', () => {
  const all = Object.keys(defaultAppearance).sort()
  assert.deepEqual([...interfaceAppearanceKeys, ...messageAppearanceKeys].sort(), all)
  assert.deepEqual(appearanceDefaults(messageAppearanceKeys), {
    messageLayout: 'cozy',
    messageFont: 'interface',
    messageSize: 14,
    groupSpacing: 12,
    timeFormat: '24h',
    coloredNames: true,
    animateMedia: true,
  })
  const reset = appearanceDefaults(interfaceAppearanceKeys)
  assert.ok(!('messageLayout' in reset) && 'mode' in reset && 'motion' in reset)
})
