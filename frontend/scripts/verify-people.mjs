import assert from 'node:assert/strict'
import {
  foldPersonIdentity,
  matchingPeople,
  networkPeople,
  personByNick,
} from '../shared/utils/people.ts'
import { accountSuggestions, nickWithOtherAccount } from '../shared/utils/account-suggestions.ts'
import { defaultMask, personSuggestions } from '../shared/utils/mask-builder.ts'
import { createPeopleCache, createPeopleSearch } from '../shared/utils/people-lookup.ts'

const members = [
  {
    id: 'a:one',
    nick: '[Alice]',
    displayName: 'Alice',
    account: 'account-one',
    status: 'online',
    host: 'masked.example',
  },
  {
    id: 'b:two',
    nick: '[Alice]',
    displayName: 'Alice elsewhere',
    account: 'account-two',
    status: 'online',
  },
  { id: 'a:three', nick: 'Alfred', displayName: 'Fred', account: '', status: 'away' },
]
const conversations = [
  { networkId: 'a', memberIds: ['a:one', 'a:three'] },
  { networkId: 'b', memberIds: ['b:two'] },
]
const scoped = networkPeople(members, conversations, 'a')
assert.deepEqual(
  scoped.map((member) => member.id),
  ['a:one', 'a:three'],
)
assert.equal(personByNick(scoped, '{alice}', 'rfc1459')?.id, 'a:one')
assert.equal(personByNick(scoped, '{alice}', 'ascii'), undefined)
assert.deepEqual(
  matchingPeople(scoped, 'alf').map((member) => member.id),
  ['a:three'],
)
assert.equal(
  foldPersonIdentity('^', 'strict-rfc1459') === foldPersonIdentity('~', 'strict-rfc1459'),
  false,
)
assert.equal(accountSuggestions(scoped, 'alice').accounts[0]?.account, 'account-one')
assert.equal(nickWithOtherAccount(scoped, '[Alice]')?.account, 'account-one')
assert.equal(accountSuggestions(scoped, 'alf').unknown[0]?.nick, 'Alfred')
assert.equal(personSuggestions(scoped, [], 'alf')[0]?.person.id, 'a:three')
assert.equal(defaultMask(members[0]), '*!*@masked.example')
const cache = createPeopleCache(2, 10_000)
const scope = { accountId: 'alice', networkId: 'a', purpose: 'dm', query: 'al', limit: 8 }
const ready = { items: ['known'], source: 'known', status: 'ready', more: false }
cache.put(scope, ready)
assert.deepEqual(cache.get(scope), ready)
assert.equal(
  cache.get({ ...scope, networkId: 'b' }),
  undefined,
  'network scopes do not share results',
)
assert.equal(cache.get({ ...scope, purpose: 'party' }), undefined, 'purposes do not share results')
assert.equal(
  cache.get({ ...scope, conversationId: 'channel-2' }),
  undefined,
  'conversation scopes do not share results',
)
assert.equal(cache.get({ ...scope, accountId: 'bob' }), undefined)
assert.equal(cache.get(scope), undefined, 'switching account clears the previous account cache')
cache.put(scope, { ...ready, status: 'partial' })
assert.equal(cache.get(scope), undefined, 'partial results are not cached')
cache.put(scope, { ...ready, status: 'unavailable' })
assert.equal(cache.get(scope), undefined, 'unavailable results are not cached')
cache.put(scope, ready)
cache.put({ ...scope, query: 'bo' }, ready)
cache.put({ ...scope, query: 'ca' }, ready)
assert.equal(cache.get(scope), undefined, 'cache is bounded')
const requests = []
const states = []
const search = createPeopleSearch(
  cache,
  (request, signal) => new Promise((resolve) => requests.push({ request, signal, resolve })),
  (state) => states.push(state),
  0,
)
const tick = () => new Promise((resolve) => setTimeout(resolve, 10))
search.search(scope)
await tick()
search.search({ ...scope, query: 'ali' })
await tick()
assert.equal(requests[0].signal.aborted, true)
requests[1].resolve({ ...ready, items: ['new'] })
await tick()
requests[0].resolve({ ...ready, items: ['stale'] })
await tick()
assert.deepEqual(
  states.at(-1).page.items,
  ['new'],
  'stale response cannot replace the latest results',
)
search.search({ ...scope, query: 'pending' })
await tick()
search.search({ ...scope, accountId: 'bob', query: 'current' })
await tick()
assert.equal(requests[2].signal.aborted, true)
requests[3].resolve({ ...ready, items: ['bob-only'] })
await tick()
requests[2].resolve({ ...ready, items: ['alice-stale'] })
await tick()
assert.deepEqual(
  states.at(-1).page.items,
  ['bob-only'],
  'an old account cannot repopulate current results',
)
search.cancel()
console.log(
  'Scoped matching, account and mask semantics, cache isolation and stale cancellation passed.',
)
