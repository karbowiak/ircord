import { test } from 'node:test'
import assert from 'node:assert/strict'
import { defaultPreferences, sanitizePreferences } from '../shared/preferences'

test('media autoplay is off by default and only a real true turns it on', () => {
  assert.equal(defaultPreferences.autoplayMedia, false)
  assert.equal(sanitizePreferences(undefined).autoplayMedia, false)
  assert.equal(sanitizePreferences({ autoplayMedia: 'yes' }).autoplayMedia, false)
  assert.equal(sanitizePreferences({ autoplayMedia: 1 }).autoplayMedia, false)
  const saved = sanitizePreferences({ autoplayMedia: true, linkPreviews: 'all', extra: 'dropped' })
  assert.equal(saved.autoplayMedia, true)
  assert.equal(saved.linkPreviews, 'all')
  assert.equal('extra' in saved, false)
  assert.deepEqual(sanitizePreferences(JSON.parse(JSON.stringify(saved))), saved)
})

test('registering channels you create is on unless turned off', () => {
  assert.equal(defaultPreferences.registerChannels, true)
  assert.equal(sanitizePreferences(undefined).registerChannels, true)
  assert.equal(sanitizePreferences({ registerChannels: 'no' }).registerChannels, true)
  assert.equal(sanitizePreferences({ registerChannels: false }).registerChannels, false)
})
