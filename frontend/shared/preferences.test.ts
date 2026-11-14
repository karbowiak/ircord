import assert from 'node:assert/strict'
import test from 'node:test'
import { sanitizePreferences } from './preferences'

test('the NSFW opt-in is one account-wide switch, off unless set to true', () => {
  assert.equal(sanitizePreferences({}).showNsfw, false)
  assert.equal(sanitizePreferences({ showNsfw: 'yes' }).showNsfw, false)
  assert.equal(sanitizePreferences({ showNsfw: true }).showNsfw, true)
})
