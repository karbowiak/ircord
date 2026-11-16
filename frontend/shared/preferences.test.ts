import assert from 'node:assert/strict'
import test from 'node:test'
import { sanitizePreferences } from './preferences'

test('the NSFW opt-in is one account-wide switch, off unless set to true', () => {
  assert.equal(sanitizePreferences({}).showNsfw, false)
  assert.equal(sanitizePreferences({ showNsfw: 'yes' }).showNsfw, false)
  assert.equal(sanitizePreferences({ showNsfw: true }).showNsfw, true)
})

test('the channel guide notice stays until it is marked seen', () => {
  assert.equal(sanitizePreferences({}).channelGuideSeen, false)
  assert.equal(sanitizePreferences({ channelGuideSeen: 1 }).channelGuideSeen, false)
  assert.equal(sanitizePreferences({ channelGuideSeen: true }).channelGuideSeen, true)
})
