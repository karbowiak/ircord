import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  explicitDisplayName,
  fallbackDisplayName,
  memberNameColor,
  selfForNetwork,
} from '../shared/utils/identity'
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
  assert.equal(selfForNetwork(self, 'Karbowiak', 'Michael Madsen')?.displayName, 'Michael Madsen')
  assert.equal(selfForNetwork(self, 'Karbowiak', 'IRCord')?.displayName, 'Karbowiak')
  assert.equal(
    selfForNetwork(
      { ...self, displayName: 'You', nameSource: 'profile' },
      'Karbowiak',
      'Michael Madsen',
    )?.displayName,
    'You',
  )
  assert.equal(
    selfForNetwork({ ...self, displayName: 'Custom profile' }, 'Karbowiak', 'Michael Madsen')
      ?.displayName,
    'Custom profile',
  )
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

test('an unset profile name follows the nickname and is never treated as chosen', () => {
  const unset: Member = {
    id: 'you',
    nick: 'ircord',
    displayName: 'ircord',
    nameSource: 'nick',
    color: 'lavender',
    status: 'online',
    role: 'member',
    bio: '',
    account: '',
  }
  assert.equal(explicitDisplayName(unset), '')
  assert.equal(explicitDisplayName({ ...unset, nameSource: undefined, displayName: 'You' }), '')
  assert.equal(
    explicitDisplayName({ ...unset, nameSource: undefined, displayName: 'Legacy' }),
    'Legacy',
  )
  assert.equal(
    explicitDisplayName({ ...unset, displayName: 'Alice', nameSource: 'profile' }),
    'Alice',
  )
  // A fresh account before any network reads as “You” without storing it.
  assert.equal(selfForNetwork(unset)?.displayName, 'You')
  assert.equal(selfForNetwork(unset, 'alice')?.displayName, 'alice')
  // Nickname changes keep driving the fallback.
  assert.equal(selfForNetwork({ ...unset, nick: 'alice' }, 'alice_')?.displayName, 'alice_')
  assert.equal(
    selfForNetwork({ ...unset, displayName: 'Alice', nameSource: 'profile' }, 'alice_')
      ?.displayName,
    'Alice',
  )
})

test('the fallback ignores a saved profile name, so clearing previews the real result', () => {
  const chosen: Member = {
    id: 'you',
    nick: 'alice',
    displayName: 'Chosen name',
    nameSource: 'profile',
    color: 'lavender',
    status: 'online',
    role: 'member',
    bio: '',
    account: '',
  }
  assert.equal(fallbackDisplayName(chosen, 'alice'), 'alice')
  assert.equal(fallbackDisplayName(chosen, 'alice', 'Alice Smith'), 'Alice Smith')
  assert.equal(fallbackDisplayName(chosen, 'alice', 'IRCord'), 'alice')
  assert.equal(fallbackDisplayName({ ...chosen, nick: 'ircord' }), 'You')
})
