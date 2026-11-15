import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isLinkedMessage,
  linkMessageId,
  messageLinkInText,
  messageLinkURL,
  parseMessageLink,
} from './message-links'
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

test('links carry the shared ID that every reader on the network stores', () => {
  const sent = { id: 'local-send', upstreamId: 'abc' }
  assert.equal(linkMessageId('home', sent), 'home:abc')
  assert.equal(linkMessageId('home', { id: 'random' }), 'random')
  assert.equal(linkMessageId('home', { id: 'random', upstreamId: '_synthetic' }), 'random')
  // The sender's copy and another reader's copy both match the link.
  assert(isLinkedMessage('home', sent, 'home:abc'))
  assert(isLinkedMessage('home', { id: 'home:abc', upstreamId: 'abc' }, 'home:abc'))
  assert(isLinkedMessage('home', sent, 'local-send'))
  assert(!isLinkedMessage('other', sent, 'home:abc'))
  assert(!isLinkedMessage('home', { id: 'x', upstreamId: '_abc' }, 'home:_abc'))
})

test('reads a message link in prose without trailing punctuation or brackets', () => {
  const link = { conversationId: 'x', messageId: 'abc' }
  for (const text of [
    'https://chat.example/channel/x?m=abc',
    'https://chat.example/channel/x?m=abc.',
    'https://chat.example/channel/x?m=abc),',
    'https://chat.example/channel/x?m=abc]',
    'https://chat.example/channel/x?m=abc"',
    "https://chat.example/channel/x?m=abc'.",
  ])
    assert.deepEqual(messageLinkInText(text, origin), link, text)
  // A balanced bracket belongs to the link.
  assert.deepEqual(messageLinkInText('https://chat.example/channel/x?m=a(b)', origin), {
    conversationId: 'x',
    messageId: 'a(b)',
  })
  assert.equal(messageLinkInText('https://other.example/channel/x?m=abc', origin), null)
  assert.equal(messageLinkInText('see https://chat.example/channel/x?m=abc', origin), null)
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
