import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  groupMentioned,
  groupsIn,
  isGroupMention,
  rankFor,
  recipientIn,
  type MentionRecipient,
} from '../shared/group-mentions'
import { defaultNotificationSettings, isHighlight, notificationFor } from '../shared/notifications'
import { shouldReadAloud } from '../shared/read-aloud'

const me = (over: Partial<MentionRecipient> = {}): MentionRecipient => ({
  nick: 'alice',
  channel: true,
  away: false,
  rank: '',
  groupMentions: 'all',
  ...over,
})

test('group mentions are whole “@” words', () => {
  assert.deepEqual(groupsIn('\x02@HERE\x0f, @oper and @voiced: @everyone! (@Channel) @owner'), [
    'here',
    'operator',
    'voiced',
    'everyone',
    'channel',
    'owner',
  ])
  for (const text of ['here', 'everyone look', '@heresy', 'mail@here', '@here-ish', '@opera'])
    assert.deepEqual(groupsIn(text), [], text)
  assert.equal(isGroupMention('@oper'), true)
  assert.equal(isGroupMention('@operators'), false)
})

test('ranks follow the network PREFIX and include the ranks above them', () => {
  assert.equal(rankFor('@+', undefined), 'operator')
  assert.equal(rankFor('~', { PREFIX: '(ov)@+' }), '')
  assert.equal(groupMentioned('@voiced', me({ rank: 'owner' })), true)
  assert.equal(groupMentioned('@voiced', me({ rank: 'halfop' })), true)
  assert.equal(groupMentioned('@voiced', me()), false)
  assert.equal(groupMentioned('@oper', me({ rank: 'admin' })), true)
  assert.equal(groupMentioned('@operator', me({ rank: 'halfop' })), false)
  assert.equal(groupMentioned('@owner', me({ rank: 'admin' })), false)
  assert.equal(groupMentioned('@owner', me({ rank: 'owner', away: true })), true)
})

test('@here and @channel skip you while away; @everyone does not; DMs never match', () => {
  assert.equal(groupMentioned('@here', me()), true)
  assert.equal(groupMentioned('@channel', me({ away: true })), false)
  assert.equal(groupMentioned('@everyone', me({ away: true })), true)
  assert.equal(groupMentioned('@everyone', me({ channel: false })), false)
})

test('the channel setting gates @here, @channel and @everyone, not role mentions', () => {
  const cases = [
    ['all', false, true],
    ['all', true, true],
    ['ops', false, false],
    ['ops', true, true],
    [undefined, false, false],
    [undefined, true, true],
    ['off', false, false],
    ['off', true, false],
  ] as const
  for (const [setting, senderOperator, want] of cases) {
    for (const text of ['@here', '@channel', '@everyone'])
      assert.equal(
        groupMentioned(text, me({ groupMentions: setting, senderOperator })),
        want,
        `${text} ${setting} ${senderOperator}`,
      )
    assert.equal(
      groupMentioned('@voiced', me({ rank: 'voiced', groupMentions: setting, senderOperator })),
      true,
    )
  }
})

test('recipientIn reads your status and the sender’s from the channel', () => {
  const c = {
    kind: 'channel' as const,
    memberModes: { you: '+', op1: '@', v1: '+' },
    channelSettings: { notifications: 'default' as const, groupMentions: 'ops' as const },
  }
  const network = { nick: 'alice', isupport: { PREFIX: '(qaohv)~&@%+' } }
  const r = recipientIn(c, network, { status: 'away' }, 'op1')
  assert.deepEqual(r, {
    nick: 'alice',
    channel: true,
    away: true,
    rank: 'voiced',
    groupMentions: 'ops',
    senderOperator: true,
  })
  assert.equal(recipientIn(c, network, undefined, 'v1').senderOperator, false)
})

test('a group mention notifies and is read aloud like a personal mention', () => {
  const s = defaultNotificationSettings
  const channel = { id: 'c', kind: 'channel' as const }
  const message = (text: string, senderId = 'bob') => ({ text, kind: 'message' as const, senderId })
  const noon = new Date('2026-01-01T12:00:00Z')
  assert.equal(isHighlight(s, '@here standup', me()), true)
  assert.equal(isHighlight(s, '@here standup', 'alice'), false)
  assert.equal(notificationFor(s, channel, message('@everyone hi'), me(), 'bob', noon), 'mention')
  assert.equal(
    notificationFor(s, channel, message('@everyone hi', 'you'), me(), 'alice', noon),
    null,
  )
  assert.equal(
    notificationFor({ ...s, ignore: ['bob'] }, channel, message('@everyone hi'), me(), 'bob', noon),
    null,
  )
  assert.equal(shouldReadAloud(s, channel, message('@here deploy'), me(), 'bob', noon), true)
  assert.equal(
    shouldReadAloud(s, channel, message('@here deploy'), me({ groupMentions: 'off' }), 'bob', noon),
    false,
  )
})
