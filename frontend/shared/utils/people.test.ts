import assert from 'node:assert/strict'
import test from 'node:test'
import { createNickIndex, directPartnerOf, foldPersonIdentity } from './people'

const people = new Map([
  ['net:1', { id: 'net:1', nick: 'PeterPowers' }],
  ['net:2', { id: 'net:2', nick: 'jules' }],
])
const byId = (id: string) => people.get(id)
const byNick = (nick: string) => [...people.values()].find((p) => p.nick === nick)

test('a direct conversation names its partner by member id', () => {
  const conversation = { kind: 'direct', name: 'PeterPowers', memberIds: ['you', 'net:2'] }
  assert.equal(directPartnerOf(conversation, byId, byNick)?.nick, 'jules')
})

test('a direct conversation that lists only you is matched by its nick', () => {
  const conversation = { kind: 'direct', name: 'PeterPowers', memberIds: ['you'] }
  assert.equal(directPartnerOf(conversation, byId, byNick)?.id, 'net:1')
})

test('channels and parties are never matched by name', () => {
  assert.equal(
    directPartnerOf({ kind: 'channel', name: 'PeterPowers', memberIds: ['you'] }, byId, byNick),
    undefined,
  )
  assert.equal(
    directPartnerOf(
      { kind: 'direct', name: 'PeterPowers', memberIds: ['you'], party: {} },
      byId,
      byNick,
    ),
    undefined,
  )
})

test('the nick index groups people by folded nick, in list order, for each casemapping', () => {
  const index = createNickIndex<{ id: string; nick: string }>()
  const first = { id: 'a:1', nick: 'Peter[away]' }
  const second = { id: 'b:1', nick: 'peter{away}' }
  const byMapping = index.index([first, second], ['rfc1459', 'ascii', 'rfc1459'])
  assert.deepEqual(byMapping.get('rfc1459')?.get('peter{away}'), [first, second])
  assert.deepEqual(byMapping.get('ascii')?.get('peter[away]'), [first])
  assert.deepEqual(byMapping.get('ascii')?.get('peter{away}'), [second])
  assert.equal(byMapping.size, 2)
})

test('the nick index folds each person once per casemapping', () => {
  const index = createNickIndex<{ id: string; nick: string }>()
  const person = { id: 'a:1', nick: 'Jules' }
  assert.equal(index.fold(person, 'rfc1459'), foldPersonIdentity('Jules', 'rfc1459'))
  // A cached result survives a rename of the same object; real updates bring a new object.
  person.nick = 'Other'
  assert.equal(index.fold(person, 'rfc1459'), 'jules')
  assert.equal(index.fold({ ...person }, 'rfc1459'), 'other')
})
