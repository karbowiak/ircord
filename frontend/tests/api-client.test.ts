import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createIRCordClient, ApiError } from '../shared/api/client'
import { normalizeTransport, parseServerEvent } from '../app/utils/transport'

test('generated HTTP client encodes paths, preserves omission, cookies and abort signals', async () => {
  const seen: Request[] = []
  const api = createIRCordClient('https://chat.example/api/v1', {
    fetch: async request => {
      seen.push(request)
      return Response.json(request.method === 'GET' ? { items: [], hasMore: false } : { id: 'network' })
    },
  })
  const controller = new AbortController()
  await api.messages('room/with space', { before: 'a+b&c', limit: 100 }, controller.signal)
  assert.equal(seen[0]!.credentials, 'include')
  assert.equal(new URL(seen[0]!.url).pathname, '/api/v1/conversations/room%2Fwith%20space/messages')
  assert.equal(new URL(seen[0]!.url).searchParams.get('before'), 'a+b&c')
  controller.abort()
  assert.equal(seen[0]!.signal.aborted, true)
  await api.updateNetwork('network', { name: 'Renamed' })
  assert.deepEqual(await seen[1]!.json(), { name: 'Renamed' })
  await api.updateNetwork('network', { auth: { method: 'server-password', password: '' } })
  assert.equal((await seen[2]!.json()).auth.password, '')
})

test('client handles empty successes and structured authentication failures', async () => {
  let expired = 0
  const api = createIRCordClient('https://chat.example', {
    fetch: async request => request.url.endsWith('/logout')
      ? new Response(null, { status: 204 })
      : Response.json({ status: 401, title: 'Unauthorized', detail: 'Sign in again.' }, { status: 401 }),
    onUnauthorized: () => expired++,
  })
  assert.equal(await api.logout(), undefined)
  await assert.rejects(api.bootstrap(), error => error instanceof ApiError && error.statusCode === 401 && error.message === 'Sign in again.')
  assert.equal(expired, 1)
})

test('generated event validator checks payloads and keeps ephemeral self identity consistent', () => {
  normalizeTransport({ user: { id: 'account', nick: 'me', displayName: 'Me' } })
  assert.deepEqual(parseServerEvent('{"v":1,"type":"session.ready","cursor":12}'), { v: 1, type: 'session.ready', cursor: 12 })
  const activity = parseServerEvent(JSON.stringify({ v: 1, type: 'typing', data: { conversationId: 'room', memberId: 'account', state: 'active' } }))
  assert.equal(activity.type, 'typing')
  assert.deepEqual(activity.type === 'typing' && activity.data, { conversationId: 'room', memberId: 'you', state: 'active' })
  for (const value of [
    { v: 2, type: 'session.ready', cursor: 1 },
    { v: 1, type: 'session.ready', cursor: '1' },
    { v: 1, type: 'typing', data: { conversationId: 'room', memberId: 'peer', state: 'invalid' } },
    { v: 1, type: 'message.created', seq: 1, data: { text: 'incomplete' } },
    { v: 1, type: 'unknown' },
  ]) assert.throws(() => parseServerEvent(JSON.stringify(value)))
})

// These are compile checks, deliberately never invoked at runtime.
function rejectsInvalidInputs(api: ReturnType<typeof createIRCordClient>) {
  // @ts-expect-error only generated enum values are valid
  api.updateNetwork('id', { status: 'online' })
  // @ts-expect-error the generated request requires a command identity
  api.sendMessage('id', { text: 'missing clientId' })
  // @ts-expect-error unsupported request fields must not drift into the client
  api.updateSelf('id', { unknownField: true })
}
