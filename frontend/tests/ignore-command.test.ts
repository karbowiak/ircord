import { test } from 'node:test'
import assert from 'node:assert/strict'
import { addIgnore, hasIgnore, ignoreCommand, removeIgnore } from '../shared/utils/ignore-command'
import { notificationLimits } from '../shared/notifications'

test('/ignore <nick> adds the nick to the ignore list', () => {
  const out = ignoreCommand('ignore', 'Jules', ['spam!*@*'])
  assert.deepEqual(out.ignore, ['spam!*@*', 'Jules'])
  assert.match(out.notice, /Ignoring Jules/)
})

test('/ignore drops a leading @ and accepts masks', () => {
  assert.deepEqual(ignoreCommand('ignore', '@jules', []).ignore, ['jules'])
  assert.deepEqual(ignoreCommand('ignore', '*!*@bad.host', []).ignore, ['*!*@bad.host'])
})

test('/ignore of someone already ignored changes nothing', () => {
  const out = ignoreCommand('ignore', 'JULES', ['jules'])
  assert.equal(out.ignore, undefined)
  assert.match(out.notice, /already ignoring/)
})

test('/unignore removes the entry case-insensitively', () => {
  const out = ignoreCommand('unignore', 'jules', ['Jules', 'max'])
  assert.deepEqual(out.ignore, ['max'])
  assert.match(out.notice, /No longer ignoring jules/)
})

test('/unignore of someone not ignored changes nothing', () => {
  const out = ignoreCommand('unignore', 'jules', ['max'])
  assert.equal(out.ignore, undefined)
  assert.match(out.notice, /aren’t ignoring jules/)
})

test('/ignore with no argument lists who is ignored', () => {
  assert.match(ignoreCommand('ignore', '', ['jules', 'max']).notice, /2 people: jules, max/)
  assert.match(ignoreCommand('ignore', '  ', []).notice, /aren’t ignoring anyone/)
  assert.equal(ignoreCommand('ignore', '', ['jules']).ignore, undefined)
})

test('/unignore without a nick shows usage', () => {
  assert.match(ignoreCommand('unignore', '', ['jules']).notice, /Usage/)
})

test('/ignore with several words shows usage and changes nothing', () => {
  const out = ignoreCommand('ignore', 'jules spammer', [])
  assert.equal(out.ignore, undefined)
  assert.match(out.notice, /Usage: \/ignore/)
})

test('/ignore refuses entries the saved list would drop', () => {
  const long = ignoreCommand('ignore', 'x'.repeat(notificationLimits.ignoreEntry + 1), [])
  assert.equal(long.ignore, undefined)
  assert.match(long.notice, /too long/)
  const full = Array.from({ length: notificationLimits.ignore }, (_, i) => `n${i}`)
  const out = ignoreCommand('ignore', 'jules', full)
  assert.equal(out.ignore, undefined)
  assert.match(out.notice, /maximum/)
})

test('/ignore refuses to hide your own messages', () => {
  const self = { nick: 'Karbowiak', user: 'k', host: 'home' }
  for (const target of ['karbowiak', '*', '*!*@home']) {
    const out = ignoreCommand('ignore', target, [], { self })
    assert.equal(out.ignore, undefined, target)
    assert.match(out.notice, /your own messages/)
  }
  assert.deepEqual(ignoreCommand('ignore', 'jules', [], { self }).ignore, ['jules'])
})

test('a mask that already hides someone is named instead of a false answer', () => {
  const person = { nick: 'jules', user: 'j', host: 'bad.host' }
  const list = ['*!*@bad.host']
  const ignore = ignoreCommand('ignore', 'jules', list, { person })
  assert.equal(ignore.ignore, undefined)
  assert.match(ignore.notice, /already hidden by \*!\*@bad\.host/)
  const unignore = ignoreCommand('unignore', 'jules', list, { person })
  assert.equal(unignore.ignore, undefined)
  assert.match(unignore.notice, /hidden by \*!\*@bad\.host; \/unignore \*!\*@bad\.host/)
})

test('list helpers shared with the user menu', () => {
  assert.equal(hasIgnore(['Jules'], 'jules'), true)
  assert.deepEqual(addIgnore(['Jules'], 'jules'), ['Jules'])
  assert.deepEqual(removeIgnore(['Jules', 'max'], 'JULES'), ['max'])
})
