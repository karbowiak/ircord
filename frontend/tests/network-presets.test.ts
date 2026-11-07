import { test } from 'node:test'
import assert from 'node:assert/strict'
import { networkPresets } from '../shared/network-presets'

test('public network presets configure connection hints without copying credentials', () => {
  const [preset] = networkPresets(JSON.stringify([{ name: 'IRCord', host: 'irc.ircord.com', port: 6697, tls: true, authMethod: 'server-password', autoJoin: ['#general'], password: 'not-part-of-the-preset' }]))
  assert.deepEqual(preset, { name: 'IRCord', host: 'irc.ircord.com', port: 6697, tls: true, authMethod: 'server-password', autoJoin: ['#general'], color: 'lavender', initial: 'I', passwordProvided: false })
})

test('preset configuration rejects malformed values and supports an explicit empty list', () => {
  assert.deepEqual(networkPresets('[]'), [])
  assert.deepEqual(networkPresets('bad JSON'), [])
  assert.deepEqual(networkPresets([{ name: 'Bad', host: 'https://example.org', port: 6697 }]), [])
  assert.deepEqual(networkPresets([{ name: 'Bad', host: 'irc.example.org', port: 70000 }]), [])
  assert.equal(networkPresets('').length, 4)
  const values = networkPresets([{ name: 'Local', host: 'irc.example.org', port: 6667, tls: false }, { name: 'Duplicate', host: 'IRC.EXAMPLE.ORG', port: 6667 }])
  assert.equal(values.length, 1)
  assert.equal(values[0]?.tls, false)
})
