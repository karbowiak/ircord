import { beforeEach, expect, mock, test } from 'bun:test'

mock.module('../../stores/chat', () => ({ serverViewId: (id) => `server:${id}` }))

const auth = { account: { id: 'account-a' } }
let notices
let sent
let rawResult
let sendResult
let directResult
let joinResult
let gifs
const chat = {
  active: { id: 'origin', kind: 'channel', name: 'general', networkId: 'network-a' },
  activeId: 'origin',
  networkId: 'network-a',
  network: { id: 'network-a', isupport: {}, features: {} },
  memberByNick: (name) => (name === 'Sam' ? { id: 'member-sam' } : undefined),
  notify: (message) => notices.push(message),
  raw: () => rawResult,
  send: (...args) => (sent.push(args), sendResult),
  activate: (id) => {
    chat.activeId = id
  },
  direct: { mutateAsync: (input) => directResult(input) },
  join: { mutateAsync: (input) => joinResult(input) },
}

Object.assign(globalThis, {
  reactive: (value) => value,
  useI18n: () => ({ t: (key) => key }),
  useChatStore: () => chat,
  useAuthStore: () => auth,
  useMediaStore: () => ({ appKey: '' }),
  useWorkspaceActions: () => ({}),
  errorText: (error) => String(error),
  klipyGifs: async () => gifs,
  klipyShare: () => {},
})
const { useCommands } = await import('./useCommands')

beforeEach(() => {
  notices = []
  sent = []
  rawResult = true
  sendResult = true
  auth.account = { id: 'account-a' }
  chat.networkId = 'network-a'
  chat.activeId = 'origin'
  directResult = async () => ({ id: 'dm-sam', networkId: 'network-a' })
  joinResult = async () => ({ id: 'joined', networkId: 'network-a' })
  gifs = { items: [{ url: 'https://example.test/gif' }] }
})

test('failed nickname and raw commands keep their draft', async () => {
  const { run } = useCommands()
  expect(await run('/nick invalid nick')).toBe('kept')
  rawResult = false
  for (const input of [
    '/nick valid',
    '/topic words',
    '/away busy',
    '/back',
    '/kick Sam why',
    '/ban Sam',
    '/invite Sam',
    '/mode +o Sam',
    '/whois Sam',
    '/quote PING',
    '/msg stranger hello',
  ])
    expect(await run(input)).toBe('kept')
  rawResult = true
  expect(await run('/nick valid')).toBe('handled')
})

test('delayed direct sends only to the returned conversation after navigation', async () => {
  let finish
  let input
  directResult = (value) => {
    input = value
    return new Promise((resolve) => {
      finish = resolve
    })
  }
  const reply = { id: 'reply', conversationId: 'dm-sam' }
  const pending = useCommands().run('/msg Sam hello  there', { replyTo: reply })
  chat.activeId = 'another-channel'
  finish({ id: 'dm-sam', networkId: 'network-a' })
  expect(await pending).toBe('handled')
  expect(input).toEqual({ memberId: 'member-sam', networkId: 'network-a', guardSelection: true })
  expect(sent).toEqual([['hello  there', reply, undefined, { conversationId: 'dm-sam' }]])
  expect(chat.activeId).toBe('another-channel')
})

test('delayed direct sends stop after account or network changes', async () => {
  for (const change of [
    () => {
      auth.account = { id: 'account-b' }
    },
    () => {
      chat.networkId = 'network-b'
    },
  ]) {
    let finish
    directResult = () =>
      new Promise((resolve) => {
        finish = resolve
      })
    const pending = useCommands().run('/msg Sam hello')
    change()
    finish({ id: 'dm-sam', networkId: 'network-a' })
    expect(await pending).toBe('kept')
    expect(sent).toEqual([])
    auth.account = { id: 'account-a' }
    chat.networkId = 'network-a'
  }
})

test('failed direct send keeps the draft without changing the user selection', async () => {
  sendResult = false
  chat.activeId = 'another-channel'
  expect(await useCommands().run('/msg Sam hello')).toBe('kept')
  expect(chat.activeId).toBe('another-channel')
  expect(sent[0][3]).toEqual({ conversationId: 'dm-sam' })
})

test('reports a direct reply only when the returned conversation used it and the send succeeded', async () => {
  const used = []
  const onReplyUsed = (id) => used.push(id)
  const reply = { id: 'reply', conversationId: 'dm-sam' }
  expect(await useCommands().run('/msg Sam hello', { replyTo: reply, onReplyUsed })).toBe('handled')
  expect(used).toEqual(['dm-sam'])
  used.length = 0
  expect(
    await useCommands().run('/msg Sam hello', {
      replyTo: { ...reply, conversationId: 'another' },
      onReplyUsed,
    }),
  ).toBe('handled')
  expect(used).toEqual([])
  sendResult = false
  expect(await useCommands().run('/msg Sam hello', { replyTo: reply, onReplyUsed })).toBe('kept')
  expect(used).toEqual([])
})

test('reports a GIF reply only after a successful send', async () => {
  const used = []
  const onReplyUsed = (id) => used.push(id)
  const reply = { id: 'reply', conversationId: 'origin' }
  expect(await useCommands().run('/gif hello', { replyTo: reply, onReplyUsed })).toBe('handled')
  expect(sent.at(-1)[1]).toEqual(reply)
  expect(used).toEqual(['origin'])
  used.length = 0
  sendResult = false
  expect(await useCommands().run('/gif hello', { replyTo: reply, onReplyUsed })).toBe('kept')
  expect(used).toEqual([])
  sendResult = true
  expect(await useCommands().run('/gif hello', { onReplyUsed })).toBe('handled')
  expect(used).toEqual([])
})

test('join reports a localized generic failure without echoing a key or server error', async () => {
  joinResult = async () => {
    throw new Error('server echoed secret-key')
  }
  expect(await useCommands().run('/join #private secret-key')).toBe('kept')
  expect(notices).toEqual(['workspaceUi.commandEditor.failed'])
})
