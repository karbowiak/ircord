import assert from 'node:assert/strict'
import { createFixtures } from '../server/utils/fixtures.ts'
import { archiveState } from '../shared/offline.ts'
import { defaultNotificationSettings, levelFor, notificationFor } from '../shared/notifications.ts'
import { createIRCordClient } from '../shared/api/client.ts'
import { channelRights } from '../shared/utils/irc.ts'
import { partyName } from '../shared/utils/party.ts'
import { startupChannels, startupConversation } from '../shared/utils/startup.ts'
import { conversationPath } from '../shared/utils/workspace-routes.ts'

const state = createFixtures().bootstrap
const network = state.networks[0]
const party = {
  id: 'party-1',
  conversationId: 'party-conversation',
  networkId: network.id,
  name: '',
  description: '',
  ownerId: 'alice',
  state: 'active',
  revision: 3,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  members: [
    {
      id: 'principal-alice',
      accountId: 'alice',
      backendAccountId: 'alice',
      servicesAccount: 'alice',
      servicesRegisteredAt: '2026-01-01T00:00:00Z',
      username: 'alice',
      displayName: 'Alice',
      joinedAt: '2026-01-01T00:00:00Z',
    },
    {
      id: 'principal-bob',
      servicesAccount: 'bob',
      servicesRegisteredAt: '2026-01-01T00:00:00Z',
      username: 'bob',
      displayName: 'Bob',
      joinedAt: '2026-01-02T00:00:00Z',
    },
  ],
}
const conversation = {
  ...state.conversations[0],
  id: party.conversationId,
  networkId: network.id,
  kind: 'channel',
  name: 'secret-wire-name',
  target: '#secret-wire-name',
  party,
}
state.conversations.push(conversation)
assert.equal(partyName(party, 'alice', 'Group conversation'), 'Bob')
assert.equal(conversationPath(conversation.id, state.conversations), '/dm/party-conversation')
assert(!startupChannels(state).some((c) => c.id === conversation.id))
assert.equal(
  startupConversation(state, { mode: 'last', channelId: null }, conversation.id),
  conversation.id,
)
assert.equal(
  channelRights('you', { ...conversation, memberModes: { you: '@' } }, network).op,
  false,
)
assert.equal(
  levelFor(
    { ...defaultNotificationSettings, directDefault: 'all', channelDefault: 'none' },
    conversation,
  ),
  'all',
)
assert.equal(
  levelFor({ ...defaultNotificationSettings, levels: { [conversation.id]: 'none' } }, conversation),
  'none',
)
assert.equal(
  notificationFor(
    { ...defaultNotificationSettings, directDefault: 'mentions' },
    conversation,
    { senderId: 'bob', kind: 'message', text: 'hello' },
    undefined,
    'bob',
  ),
  'mention',
)

const fences = {}
const originalCursor = state.cursor
const removed = archiveState(
  state,
  {
    v: 1,
    seq: originalCursor + 1,
    type: 'party.removed',
    data: { id: party.id, conversationId: conversation.id, revision: 4 },
  },
  fences,
)
assert(!removed.conversations.some((c) => c.id === conversation.id))
const stale = archiveState(
  removed,
  { v: 1, seq: originalCursor + 2, type: 'conversation.updated', data: conversation },
  fences,
)
assert(
  !stale.conversations.some((c) => c.id === conversation.id),
  'a stale event cannot restore membership',
)
const accepted = { ...conversation, party: { ...party, revision: 5 } }
const restored = archiveState(
  stale,
  { v: 1, seq: originalCursor + 3, type: 'conversation.updated', data: accepted },
  fences,
)
assert(
  restored.conversations.some((c) => c.id === conversation.id),
  'a later accepted membership can return',
)

const requests = []
const client = createIRCordClient('http://fixture.test/api/v1', {
  fetch: async (request) => {
    requests.push({
      method: request.method,
      path: new URL(request.url).pathname,
      body: request.method === 'GET' ? undefined : await request.text(),
    })
    return Response.json(party)
  },
})
await client.createParty({ clientId: 'request-1', servicesAccounts: ['bob'] })
await client.inviteParty(party.id, 'bob')
await client.removePartyMember(party.id, 'bob')
await client.transferParty(party.id, 'principal-bob')
assert.deepEqual(
  requests.map(({ method, path }) => [method, path]),
  [
    ['POST', '/api/v1/parties'],
    ['POST', '/api/v1/parties/party-1/invitations'],
    ['DELETE', '/api/v1/parties/party-1/members/bob'],
    ['POST', '/api/v1/parties/party-1/owner'],
  ],
)
assert.equal(JSON.parse(requests[0].body).clientId, 'request-1')
assert.deepEqual(JSON.parse(requests[0].body).servicesAccounts, ['bob'])
assert.equal(JSON.parse(requests[1].body).servicesAccount, 'bob')
assert.equal('accountIds' in JSON.parse(requests[0].body), false)
assert.equal('accountId' in JSON.parse(requests[1].body), false)
assert.equal(JSON.parse(requests[3].body).memberId, 'principal-bob')
console.log(
  'Party naming, routing, permissions, notification defaults, removal fencing and typed requests passed.',
)
