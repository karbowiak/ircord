import { test } from 'node:test'
import assert from 'node:assert/strict'
import { nicknameError, nicknameLimit } from '../shared/utils/nickname'

test('nickname validation accepts IRC symbols and rejects spaces, unsupported symbols and invalid leading characters', () => {
  for (const value of ['Alice', 'alice-123', '[alice]', '_alice', '`alice', 'a'.repeat(32)])
    assert.equal(nicknameError(value), '', value)
  for (const value of [
    '',
    'Alice Smith',
    ' Alice',
    'Alice\n',
    '9alice',
    '-alice',
    'alice@example',
    'alice😀',
    '?alice',
    'a'.repeat(33),
  ])
    assert.notEqual(nicknameError(value), '', value)
  assert.match(nicknameError('Alice Smith'), /display name/i)
  assert.equal(nicknameLimit({ NICKLEN: '20' }), 20)
  assert.notEqual(nicknameError('a'.repeat(21), nicknameLimit({ NICKLEN: '20' })), '')
  for (const value of ['', 'NaN', '-1', '0', '1000'])
    assert.equal(nicknameLimit({ NICKLEN: value }), 32)
})
