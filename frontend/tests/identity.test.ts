import { test } from 'node:test'
import assert from 'node:assert/strict'
import { memberNameColor, selfForNetwork } from '../shared/utils/identity'
import type { Member } from '../shared/types/chat'

test('self uses the selected network nickname while explicit profile names are preserved', () => {
  const self: Member = {
    id: 'you',
    nick: 'ircord',
    displayName: 'You',
    color: 'lavender',
    status: 'online',
    role: 'member',
    bio: '',
    account: '',
  }
  assert.equal(selfForNetwork(self, 'Karbowiak')?.displayName, 'Karbowiak')
  assert.equal(selfForNetwork(self, 'second-network')?.nick, 'second-network')
  assert.equal(
    selfForNetwork({ ...self, displayName: 'Custom profile' }, 'Karbowiak')?.displayName,
    'Custom profile',
  )
  assert.equal(self.displayName, 'You')
})

test('automatic name colors remain stable when a display name changes', () => {
  const identity = { id: 'network:123', account: 'mika' }
  const before = memberNameColor(identity)
  assert.equal(memberNameColor({ ...identity }), before)
  assert.ok(
    new Set(
      Array.from({ length: 30 }, (_, i) =>
        memberNameColor({ id: `person-${i}`, account: `person-${i}` }),
      ),
    ).size > 4,
  )
})
test('profile color overrides automatic colors and invalid colors fall back safely', () => {
  const identity = { id: 'you', account: 'karbowiak' }
  assert.equal(memberNameColor({ ...identity, nameColor: '#ee88cc' }), '#ee88cc')
  assert.equal(
    memberNameColor({ ...identity, nameColor: 'url(unsafe)' }),
    memberNameColor(identity),
  )
  assert.notEqual(memberNameColor({ ...identity, nameColor: '#ffffff' }, 'light'), '#ffffff')
  assert.notEqual(memberNameColor({ ...identity, nameColor: '#000000' }, 'dark'), '#000000')
})
