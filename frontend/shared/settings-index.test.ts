import assert from 'node:assert/strict'
import test from 'node:test'
import { settingsDestination, type SettingsAccess } from './settings-index'

const access: SettingsAccess = {
  emoteAdd: false,
  push: false,
  quietHours: false,
  install: false,
  localSpeech: true,
  localSpeechOn: false,
  speechServiceOn: true,
}

test('the voice model settings open its switch until it is on', () => {
  assert.deepEqual(settingsDestination('local-voice', access), {
    section: 'notifications',
    setting: 'local-speech',
  })
  assert.deepEqual(settingsDestination('local-voice', { ...access, localSpeechOn: true }), {
    section: 'notifications',
    setting: 'local-voice',
  })
})

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

test('the speech service settings open the voice source selector while the voice model is chosen', () => {
  const modelChosen = { ...access, localSpeechOn: true, speechServiceOn: false }
  for (const id of ['speech-service', 'speech-test', 'speech-remove']) {
    assert.deepEqual(settingsDestination(id, modelChosen), {
      section: 'notifications',
      setting: 'local-speech',
    })
    assert.deepEqual(settingsDestination(id, access), { section: 'notifications', setting: id })
  }
})
