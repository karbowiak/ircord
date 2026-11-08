import { test } from 'node:test'
import assert from 'node:assert/strict'
import type { Message } from '../shared/types/chat'
import { chatterSummary, eventReason, isChatter, modeDescription } from '../shared/utils/events'
import { eventDescription } from '../shared/utils/log'

const event = (type: string, actorId: string, extra: Record<string, string> = {}) =>
  ({
    id: `${type}-${actorId}`,
    conversationId: 'general',
    senderId: actorId,
    text: '',
    createdAt: new Date().toISOString(),
    kind: 'event',
    reactions: [],
    event: { type, actorId, ...extra },
  }) as Message
const nick = (id: string) => id

test('mode changes read as sentences', () => {
  assert.equal(modeDescription('+o sage'), 'made sage an operator')
  assert.equal(
    modeDescription('-o+v sage noah'),
    'removed operator from sage and gave voice to noah',
  )
  assert.equal(modeDescription('+b *!*@spam.example'), 'banned *!*@spam.example')
  assert.equal(modeDescription('-b *!*@spam.example'), 'unbanned *!*@spam.example')
  assert.equal(modeDescription('+mi'), 'moderated the channel and made the channel invite-only')
  assert.equal(modeDescription('+l 20'), 'limited the channel to 20 users')
  assert.equal(modeDescription('-l'), 'removed the user limit')
  // The key itself is never shown.
  assert.equal(modeDescription('+k hunter2'), 'set a channel key')
  // Arguments stay matched to their modes past an unknown flag and a key.
  assert.equal(
    modeDescription('+kxo secret sage'),
    'set a channel key, made sage an operator and set mode +x',
  )
  assert.equal(modeDescription('+Z'), 'set mode +Z')
})

test('reasons drop the empty quit boilerplate', () => {
  assert.equal(eventReason('Quit: '), '')
  assert.equal(eventReason('Client Quit'), '')
  assert.equal(eventReason('Quit: gone fishing'), 'gone fishing')
  assert.equal(eventReason('Ping timeout: 245 seconds'), 'Ping timeout: 245 seconds')
  assert.equal(eventDescription(event('quit', 'ellis', { text: 'Quit: ' }), nick), 'disconnected')
  assert.equal(
    eventDescription(event('kick', 'mika', { targetId: 'ellis', text: 'spam' }), nick, false),
    'kicked ellis',
  )
})

test('only joins, parts, quits and nick changes are chatter', () => {
  assert.ok(isChatter(event('join', 'ada')))
  assert.ok(isChatter(event('nick', 'ada', { text: 'ada_' })))
  assert.ok(!isChatter(event('kick', 'mika')))
  assert.ok(!isChatter(event('mode', 'mika', { text: '+o sage' })))
  assert.ok(!isChatter(event('topic', 'mika', { text: 'hi' })))
})

test('chatter summarises by outcome', () => {
  const split = [
    event('quit', 'jules', { text: '*.net *.split' }),
    event('quit', 'noah', { text: '*.net *.split' }),
    event('quit', 'sage', { text: '*.net *.split' }),
    event('join', 'jules'),
    event('join', 'noah'),
    event('join', 'sage'),
    event('join', 'ellis'),
    event('part', 'lin'),
    event('join', 'pat'),
    event('part', 'pat'),
    event('nick', 'ada', { text: 'ada_away' }),
  ]
  assert.equal(
    chatterSummary(split, nick),
    'jules, noah and sage reconnected · ellis joined · pat joined and left · lin left · ada → ada_away',
  )
  const crowd = ['a', 'b', 'c', 'd', 'e'].map((id) => event('join', id))
  assert.equal(chatterSummary(crowd, nick), '5 people joined')
})

test('automatic channel registration stands on its own line', () => {
  const done = event('registered', 'you')
  assert.ok(!isChatter(done))
  assert.equal(
    eventDescription(done, nick),
    'created this channel, so it’s registered to your account',
  )
  assert.equal(
    eventDescription(
      event('registration-failed', 'you', {
        text: 'You have reached this network’s channel registration limit.',
      }),
      nick,
    ),
    'created this channel, but it couldn’t be registered to your account. You have reached this network’s channel registration limit.',
  )
})
