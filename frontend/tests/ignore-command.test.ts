import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ignoreCommand } from '../shared/utils/ignore-command'

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
