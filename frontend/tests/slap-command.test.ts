import { test } from 'node:test'
import assert from 'node:assert/strict'
import { slapAction } from '../shared/utils/commands'

test('/slap sends the classic action with a cucumber, not a trout', () => {
  assert.equal(slapAction('jules'), 'slaps jules around a bit with a large cucumber')
  assert.doesNotMatch(slapAction('jules'), /trout/)
})
