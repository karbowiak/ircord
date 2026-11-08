import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createFixtures } from '../server/utils/fixtures'
import {
  conversationSendReason,
  messageActionReason,
  messageTextReason,
  NEEDS_ACCOUNT,
  OTHER_ACCOUNT,
} from '../shared/utils/features'
import { channelRights } from '../shared/utils/irc'
import { parseServerEvent } from '../app/utils/transport'

test('live network feature snapshots satisfy the generated WebSocket contract', () => {
  const { bootstrap } = createFixtures()
  const network = bootstrap.networks[0]!
  const event = { v: 1, seq: 1, type: 'network.updated', data: network }
  assert.equal(parseServerEvent(JSON.stringify(event)).type, 'network.updated')
  const invalid = structuredClone(event)
  invalid.data.features.multiline!.maxLines = 0
  assert.throws(() => parseServerEvent(JSON.stringify(invalid)))
})

test('network features, membership, IDs and connection state agree for message actions', () => {
  const { bootstrap, messages } = createFixtures()
  const c = bootstrap.conversations.find((c) => c.id === 'general')!
  const n = bootstrap.networks.find((n) => n.id === c.networkId)!
  const message = messages.general!.find((m) => m.kind === 'message')!
  const limited = bootstrap.networks.find((n) => n.id === 'oftc')!
  assert.equal(messageActionReason(message, c, n, 'replies'), undefined)
  assert.equal(messageActionReason(message, c, n, 'reactions'), undefined)
  assert.match(messageActionReason(message, c, limited, 'replies')!, /unavailable/)
  assert.match(
    messageActionReason(message, c, { ...n, status: 'connecting' }, 'replies')!,
    /Reconnect/,
  )
  assert.match(messageActionReason(message, c, n, 'reactions', false)!, /Reconnect/)
  assert.match(messageActionReason(message, { ...c, joined: false }, n, 'replies')!, /Join/)
  assert.match(
    messageActionReason({ ...message, upstreamId: undefined }, c, n, 'replies')!,
    /message ID/,
  )
  assert.match(
    messageActionReason({ ...message, upstreamId: '_history' }, c, n, 'reactions')!,
    /message ID/,
  )
  assert.match(messageActionReason({ ...message, deleted: true }, c, n, 'reactions')!, /message ID/)
  assert.match(messageActionReason(message, c, n, 'redact')!, /own messages/)
  // History is untouched, even when a network loses its sending features.
  assert.equal(message.text, messages.general!.find((m) => m.id === message.id)!.text)
})

test('effective multiline limits count UTF-8, CTCP wrappers and concat wire lines', () => {
  const { bootstrap } = createFixtures()
  const c = bootstrap.conversations.find((c) => c.id === 'general')!
  const n = bootstrap.networks.find((n) => n.id === c.networkId)!
  n.features.messageLineBytes = 43 // PRIVMSG #general : takes 18 bytes; 25 remain
  delete n.features.multiline
  assert.equal(messageTextReason('a'.repeat(25), 'message', c, n), undefined)
  assert.match(messageTextReason('a'.repeat(26), 'message', c, n)!, /one line/)
  assert.match(messageTextReason('one\ntwo', 'message', c, n)!, /multiline/)
  assert.match(messageTextReason('a'.repeat(20), 'action', c, n)!, /one line/)
  n.features.multiline = { maxBytes: 54, maxLines: 3 }
  assert.equal(messageTextReason('界'.repeat(18), 'message', c, n), undefined)
  n.features.multiline.maxLines = 2
  assert.match(messageTextReason('界'.repeat(18), 'message', c, n)!, /2 wire lines/)
  n.features.multiline.maxBytes = 50
  assert.match(messageTextReason('界'.repeat(18), 'message', c, n)!, /50 UTF-8 bytes/)
})

test('channel rights respond to live permissions, invite-only mode and network loss', () => {
  const { bootstrap } = createFixtures()
  const c = { ...bootstrap.conversations[0]!, joined: true, modes: 'it', memberModes: { you: '' } }
  const n = bootstrap.networks[0]!
  assert.equal(channelRights('you', c, n).invite, false)
  assert.equal(channelRights('you', c, n).topic, false)
  c.memberModes.you = '@'
  assert.equal(channelRights('you', c, n).invite, true)
  assert.equal(channelRights('you', c, n).kick, true)
  assert.equal(channelRights('you', { ...c, joined: false }, n).kick, false)
  assert.equal(channelRights('you', c, { ...n, status: 'disconnected' }).kick, false)
  const custom = { ...n, isupport: { PREFIX: '(ov)!+' } }
  assert.equal(channelRights('you', { ...c, memberModes: { you: '!' } }, custom).kick, true)
  assert.match(
    conversationSendReason({ ...c, modes: 'm', memberModes: { you: '' } }, n)!,
    /moderated/,
  )
  assert.equal(
    conversationSendReason({ ...c, modes: 'm', memberModes: { you: '+' } }, n),
    undefined,
  )
})

test('Delete is offered only where the network would accept the redaction', () => {
  const { bootstrap, messages } = createFixtures()
  const general = bootstrap.conversations.find((c) => c.id === 'general')!
  const network = bootstrap.networks.find((n) => n.id === general.networkId)!
  const mine = { ...messages.general!.find((m) => m.senderId === 'you' && m.kind === 'message')!, upstreamId: 'm1' }
  const as = (account: string) => ({ ...network, account })
  const modes = (you: string) => ({ ...general, memberModes: { ...general.memberModes, you } })
  const direct = { ...general, kind: 'direct' as const, memberModes: {}, modes: '' }

  // Ergo checks the account a message was sent from, so the login must match it.
  assert.equal(messageActionReason({ ...mine, account: 'Alice' }, modes(''), as('alice'), 'redact'), undefined)
  assert.equal(messageActionReason({ ...mine, account: 'alice' }, direct, as('alice'), 'redact'), undefined)
  for (const account of [undefined, 'bob'])
    assert.equal(messageActionReason({ ...mine, account }, modes(''), as('alice'), 'redact'), OTHER_ACCOUNT)
  // A shared server password or a bare nick is not an account login: Ergo refuses those.
  for (const c of [modes(''), modes('+'), modes('%'), direct])
    assert.equal(messageActionReason({ ...mine, account: undefined }, c, as(''), 'redact'), NEEDS_ACCOUNT)
  // Channel operators and above may delete in their channel without an account.
  for (const mode of ['@', '&', '~'])
    assert.equal(messageActionReason({ ...mine, account: undefined }, modes(mode), as(''), 'redact'), undefined)
})
