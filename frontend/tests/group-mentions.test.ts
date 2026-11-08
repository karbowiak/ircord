import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  groupMentioned,
  groupsIn,
  isGroupMention,
  statusAtLeast,
  recipientIn,
  type MentionRecipient,
} from '../shared/group-mentions'
import { defaultNotificationSettings, isHighlight, notificationFor } from '../shared/notifications'
import { shouldReadAloud } from '../shared/read-aloud'

const me = (over: Partial<MentionRecipient> = {}): MentionRecipient => ({
  nick: 'alice',
  channel: true,
  away: false,
  owner: false,
  operator: false,
  voiced: false,
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

test('ranks follow the network PREFIX by position and include the ranks above them', () => {
  const c = { memberModes: { op: '@+', bang: '!', tilde: '~' } }
  assert.equal(statusAtLeast('op', c, undefined, 'o'), true)
  assert.equal(statusAtLeast('op', c, undefined, 'q'), false)
  // A mode above op under another letter still counts as op and voiced, like the backend.
  const odd = { isupport: { PREFIX: '(yqaohv)!~&@%+' } }
  assert.equal(statusAtLeast('bang', c, odd, 'o'), true)
  assert.equal(statusAtLeast('bang', c, odd, 'v'), true)
  // “~” means nothing on a network that does not define it.
  assert.equal(statusAtLeast('tilde', c, { isupport: { PREFIX: '(ov)@+' } }, 'o'), false)
  const owner = me({ owner: true, operator: true, voiced: true })
  const op = me({ operator: true, voiced: true })
  assert.equal(groupMentioned('@voiced', owner), true)
  assert.equal(groupMentioned('@voiced', me({ voiced: true })), true)
  assert.equal(groupMentioned('@voiced', me()), false)
  assert.equal(groupMentioned('@oper', op), true)
  assert.equal(groupMentioned('@operator', me({ voiced: true })), false)
  assert.equal(groupMentioned('@owner', op), false)
  assert.equal(groupMentioned('@owner', me({ owner: true, away: true, offline: true })), true)
})

test('@here and @channel skip you while away; @everyone does not; DMs never match', () => {
  assert.equal(groupMentioned('@here', me()), true)
  assert.equal(groupMentioned('@channel', me({ away: true })), false)
  assert.equal(groupMentioned('@everyone', me({ away: true })), true)
  assert.equal(groupMentioned('@everyone', me({ channel: false })), false)
  // History: you weren't there for @here and @everyone.
  assert.equal(groupMentioned('@everyone', me({ offline: true })), false)
  assert.equal(groupMentioned('@here', me({ offline: true })), false)
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
      groupMentioned('@voiced', me({ voiced: true, groupMentions: setting, senderOperator })),
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
  const r = recipientIn(c, network, { status: 'away' }, { senderId: 'op1' })
  assert.deepEqual(r, {
    nick: 'alice',
    channel: true,
    away: true,
    offline: false,
    owner: false,
    operator: false,
    voiced: true,
    groupMentions: 'ops',
    senderOperator: true,
  })
  assert.equal(recipientIn(c, network, undefined, { senderId: 'v1' }).senderOperator, false)
  assert.equal(
    recipientIn(c, network, undefined, { senderId: 'op1', historical: true }).offline,
    true,
  )
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
