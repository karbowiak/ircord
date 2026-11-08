import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  accountExtban,
  expandMask,
  extbanOf,
  listInfo,
  maskMatches,
  muteExtban,
  parseExtbans,
  previewMask,
  toMask,
} from '../shared/utils/channel-modes'
import { accountSuggestions, nickWithOtherAccount } from '../shared/utils/account-suggestions'

// Ergo v2.19 advertises EXTBAN=,m; Libera (Solanum) advertises EXTBAN=$,ajrxz.
const ergo = parseExtbans({ EXTBAN: ',m' })
const solanum = parseExtbans({ EXTBAN: '$,ajrxz' })
const unreal = parseExtbans({ EXTBAN: '~,acfjmnpqrtCGOST' })
const inspircd = parseExtbans({ EXTBAN: ',ACNOQRSTUacjmnprsz' })

test('missing parts expand the way Ergo stores them', () => {
  assert.equal(expandMask('alice'), 'alice!*@*')
  assert.equal(expandMask('bob@host.example.net'), '*!bob@host.example.net')
  assert.equal(expandMask('carol!id'), 'carol!id@*')
  assert.equal(expandMask('*!*@203.0.113.7'), '*!*@203.0.113.7')
  assert.equal(expandMask('dave!@'), 'dave!*@*')
  assert.equal(toMask(' alice '), 'alice!*@*')
  assert.equal(toMask('bob@host.example.net'), '*!bob@host.example.net')
})

test('EXTBAN is parsed and only verified account masks are offered', () => {
  assert.deepEqual(ergo, { prefix: '', types: 'm' })
  assert.deepEqual(solanum, { prefix: '$', types: 'ajrxz' })
  assert.equal(parseExtbans({}), undefined)
  assert.equal(accountExtban(solanum), '$a:')
  assert.equal(accountExtban(unreal), '~a:')
  assert.equal(accountExtban(ergo), undefined)
  // InspIRCd has an `a` letter, but it doesn't mean account; don't guess.
  assert.equal(accountExtban(inspircd), undefined)
  assert.equal(muteExtban(ergo), 'm:')
  assert.equal(muteExtban(solanum), undefined)
  assert.deepEqual(extbanOf('$a:alice', solanum), { type: 'a', arg: 'alice' })
  assert.deepEqual(extbanOf('$a', solanum), { type: 'a', arg: '' })
  assert.equal(extbanOf('mallory', ergo), undefined)
  assert.equal(extbanOf('$a:alice', ergo), undefined)
})

test('extended masks follow what the network advertises', () => {
  assert.equal(toMask('$a:alice', solanum), '$a:alice')
  // Ergo reads an unadvertised $a: as a nick and expands it; the preview must say so.
  assert.equal(toMask('$a:erin', ergo), '$a:erin!*@*')
  assert.equal(toMask('m:dave', ergo), 'm:dave!*@*')
  assert.equal(toMask('mallory', ergo), 'mallory!*@*')
  // Unknown network capabilities keep the old behaviour of sending $… as typed.
  assert.equal(toMask('$a:account'), '$a:account')
})

test('the preview explains scope, shorthand and broad masks', () => {
  const nick = previewMask('alice', ergo)
  assert.equal(nick.mask, 'alice!*@*')
  assert.ok(nick.expanded)
  assert.match(nick.scope!, /the nick alice/)
  assert.deepEqual(nick.warnings, [])

  const host = previewMask('*!*@203.0.113.7', ergo)
  assert.equal(host.expanded, false)
  assert.match(host.scope!, /the host 203\.0\.113\.7/)

  assert.deepEqual(previewMask('*', ergo).warnings, ['This matches everyone, including you.'])
  assert.deepEqual(previewMask('*!*@*', ergo).warnings, ['This matches everyone, including you.'])
  assert.match(previewMask('*!*@*.com', ergo).warnings[0]!, /very broad/)
  assert.deepEqual(previewMask('*!*@*.example.net', ergo).warnings, [])

  const account = previewMask('$a:alice', solanum)
  assert.equal(account.extban, 'account')
  assert.match(account.scope!, /logged in to the account alice/)
  assert.equal(previewMask('m:dave', ergo).extban, 'mute')
  assert.match(previewMask('$a:erin', ergo).warnings[0]!, /doesn’t offer \$a: masks/)
  assert.match(previewMask('$a:erin').warnings[0]!, /doesn’t advertise extended masks/)
})

test('masks that would break the MODE command are refused', () => {
  assert.equal(previewMask('two words', ergo).error, 'Masks can’t contain spaces.')
  assert.equal(previewMask('two words', ergo).mask, '')
  assert.ok(previewMask(':alice', ergo).error)
  assert.ok(previewMask('a,b', ergo).error)
  assert.equal(previewMask('   ', ergo).mask, '')
})

test('wildcards match like IRC masks', () => {
  assert.ok(maskMatches('alice!*@*', 'Alice!~a@host.example.net'))
  assert.ok(maskMatches('*!*alice@*', 'x!~alice@h'))
  assert.ok(maskMatches('a?ice!*@*', 'alice!u@h'))
  assert.ok(!maskMatches('alice!*@*', 'alicex!u@h'))
  assert.ok(maskMatches('*!*@*.example.net', 'x!u@a.b.example.net'))
  assert.ok(!maskMatches('*!*@*.example.net', 'x!u@example.org'))
  // Regex characters in masks are literal.
  assert.ok(!maskMatches('a.c!*@*', 'abc!u@h'))
  assert.ok(maskMatches('[x]!*@*', '[x]!u@h'))
})

test('the three lists say what they override', () => {
  assert.match(listInfo.e!.overrides!, /overrides bans only/)
  assert.match(listInfo.I!.overrides!, /a ban still keeps them out/)
  assert.match(listInfo.b!.overrides!, /exceptions override bans/i)
})

const roster = [
  { id: '1', nick: 'alice_', displayName: 'Alice', account: 'alice' },
  { id: '2', nick: 'alice-phone', account: 'Alice' },
  { id: '3', nick: 'bob', account: 'robert' },
  { id: '4', nick: 'carol', account: '' },
  { id: '5', nick: 'dave', account: 'dave' },
  { id: '6', nick: 'me', account: 'karbowiak' },
]

test('suggestions are accounts, one per account, matched by nick, account or name', () => {
  const all = accountSuggestions(roster, '', { exclude: ['karbowiak'] })
  assert.deepEqual(
    all.accounts.map((a) => a.account),
    ['alice', 'dave', 'robert'],
  )
  // Several sessions of one account show up once, with every nick.
  assert.deepEqual(all.accounts[0]!.nicks, ['alice_', 'alice-phone'])
  assert.equal(all.accounts[0]!.name, 'Alice')
  assert.deepEqual(all.unknown, [])

  const bob = accountSuggestions(roster, 'bo', { exclude: ['karbowiak'] })
  assert.deepEqual(
    bob.accounts.map((a) => a.account),
    ['robert'],
  )
  assert.deepEqual(bob.accounts[0]!.nicks, ['bob'])

  // Exact account first, then account prefixes, then nick prefixes.
  const order = accountSuggestions(
    [
      { id: 'a', nick: 'zed', account: 'dan' },
      { id: 'b', nick: 'dan2', account: 'other' },
      { id: 'c', nick: 'x', account: 'danny' },
    ],
    'dan',
  )
  assert.deepEqual(
    order.accounts.map((a) => a.account),
    ['dan', 'danny', 'other'],
  )
})

test('people without a known account are explained, never offered', () => {
  const carol = accountSuggestions(roster, 'car')
  assert.deepEqual(carol.accounts, [])
  assert.deepEqual(carol.unknown, [{ nick: 'carol', name: undefined }])
  assert.deepEqual(accountSuggestions([{ id: 'x', nick: 'star', account: '*' }], 'st').accounts, [])
})

test('a typed nick that belongs to another account is pointed at that account', () => {
  assert.deepEqual(nickWithOtherAccount(roster, 'bob'), { nick: 'bob', account: 'robert' })
  assert.deepEqual(nickWithOtherAccount(roster, 'carol'), { nick: 'carol', account: undefined })
  assert.equal(nickWithOtherAccount(roster, 'dave'), undefined)
  assert.equal(nickWithOtherAccount(roster, 'robert'), undefined)
  assert.equal(nickWithOtherAccount(roster, 'nobody-here'), undefined)
  // `bob` typed while another person's account is literally bob: that account is meant.
  assert.equal(
    nickWithOtherAccount([...roster, { id: '7', nick: 'b0b', account: 'bob' }], 'bob'),
    undefined,
  )
})
