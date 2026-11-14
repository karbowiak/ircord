import { strict as assert } from 'node:assert'
import { test } from 'node:test'
import type { Member } from '../../shared/types/chat'
import { memberStatusLabel } from './member-status'

test('custom status is independent of presence and expires at its deadline', () => {
  const deadline = Date.parse('2026-11-14T12:00:00Z')
  const member = {
    status: 'offline',
    statusEmoji: ':coffee:',
    statusText: 'Taking a break',
    statusExpiresAt: new Date(deadline).toISOString(),
  } as Member
  assert.equal(memberStatusLabel(member, deadline - 1), ':coffee: Taking a break')
  assert.equal(memberStatusLabel(member, deadline), '')
  assert.equal(
    memberStatusLabel({ ...member, status: 'away', statusEmoji: '' }, deadline - 1),
    'Taking a break',
  )
  assert.equal(memberStatusLabel(undefined, deadline), '')
})
