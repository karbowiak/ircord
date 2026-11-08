import { test } from 'node:test'
import assert from 'node:assert/strict'
import { slapAction } from '../shared/utils/commands'
import { wireText } from '../shared/utils/irc'

test('/slap sends the classic action with a cucumber, not a trout', () => {
  assert.equal(slapAction('jules'), 'slaps jules around a bit with a large cucumber')
  assert.doesNotMatch(slapAction('jules'), /trout/)
})

test('a slap keeps nicks that look like markdown intact on the wire', () => {
  for (const nick of ['_jules_', '__init__', '`cat`', '*star*']) {
    assert.equal(
      wireText(slapAction(nick), true),
      `slaps ${nick} around a bit with a large cucumber`,
    )
  }
  // Ordinary messages still turn markdown into IRC formatting.
  assert.equal(wireText(' _hi_ '), '\x1dhi\x1d')
})
