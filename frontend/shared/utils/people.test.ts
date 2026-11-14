import assert from 'node:assert/strict'
import test from 'node:test'
import { directPartnerOf } from './people'

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
