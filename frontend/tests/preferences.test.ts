import { test } from 'node:test'
import assert from 'node:assert/strict'
import { defaultPreferences, sanitizePreferences } from '../shared/preferences'

test('retired preview and autoplay preferences are discarded', () => {
  const saved = sanitizePreferences({ autoplayMedia: true, linkPreviews: 'all', extra: 'dropped' })
  assert.equal('autoplayMedia' in saved, false)
  assert.equal('linkPreviews' in saved, false)
  assert.equal('extra' in saved, false)
  assert.deepEqual(sanitizePreferences(JSON.parse(JSON.stringify(saved))), saved)
})

test('registering channels you create is on unless turned off', () => {
  assert.equal(defaultPreferences.registerChannels, true)
  assert.equal(sanitizePreferences(undefined).registerChannels, true)
  assert.equal(sanitizePreferences({ registerChannels: 'no' }).registerChannels, true)
  assert.equal(sanitizePreferences({ registerChannels: false }).registerChannels, false)
})
