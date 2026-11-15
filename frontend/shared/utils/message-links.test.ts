import assert from 'node:assert/strict'
import test from 'node:test'
import { messageLinkURL, parseMessageLink } from './message-links'
import { conversationPath } from './workspace-routes'

const origin = 'https://chat.example'

test('builds a message link on the conversation route', () => {
  assert.equal(
    messageLinkURL(origin, '/channel/home_abc', 'home:x1'),
    'https://chat.example/channel/home_abc?m=home%3Ax1',
  )
  const conversations = [
    { id: 'net_1', kind: 'channel' },
    { id: 'net_2', kind: 'direct' },
    { id: 'net_3', kind: 'channel', party: { id: 'p' } },
  ] as never
  assert.equal(
    messageLinkURL(origin, conversationPath('net_2', conversations)!, 'abc'),
    'https://chat.example/dm/net_2?m=abc',
  )
  assert.equal(
    messageLinkURL(origin, conversationPath('net_3', conversations)!, 'abc'),
    'https://chat.example/dm/net_3?m=abc',
  )
})

test('reads back what it builds, including ids that need escaping', () => {
  const url = messageLinkURL(origin, `/channel/${encodeURIComponent('a b/c')}`, 'id:with/odd?&')
  assert.deepEqual(parseMessageLink(url, origin), {
    conversationId: 'a b/c',
    messageId: 'id:with/odd?&',
  })
  assert.deepEqual(parseMessageLink('https://chat.example/dm/x/?m=1&other=2', origin), {
    conversationId: 'x',
    messageId: '1',
  })
})

test('ignores links that are not messages on this origin', () => {
  for (const url of [
    'https://other.example/channel/x?m=1',
    'http://chat.example/channel/x?m=1',
    'https://chat.example/channel/x',
    'https://chat.example/channel/x?m=',
    'https://chat.example/channel/?m=1',
    'https://chat.example/server/x?m=1',
    'https://chat.example/saved?m=1',
    'https://chat.example/channel/x/y?m=1',
    'https://chat.example/channel/%E0%A4%A?m=1',
    'irc://chat.example/channel/x?m=1',
    'not a url',
  ])
    assert.equal(parseMessageLink(url, origin), null, url)
})
