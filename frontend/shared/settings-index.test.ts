import assert from 'node:assert/strict'
import test from 'node:test'
import { settingsDestination, type SettingsAccess } from './settings-index'

const access: SettingsAccess = { emoteAdd: false, push: false, quietHours: false, install: false }

test('merged sections still open, at their first setting', () => {
  assert.deepEqual(settingsDestination('devices', access), {
    section: 'account',
    setting: 'device-sessions',
  })
  assert.deepEqual(settingsDestination('read-aloud', access), {
    section: 'notifications',
    setting: 'read-aloud-device',
  })
})

test('privacy settings open in Privacy', () => {
  for (const id of ['nsfw-content', 'ask-before-embeds', 'ignored-people'])
    assert.equal(settingsDestination(id, access).section, 'privacy')
  assert.deepEqual(settingsDestination('privacy', access), { section: 'privacy' })
})
