import assert from 'node:assert/strict'
import { createReadThroughDiscovery, observedReadBoundary } from '../app/utils/read-through.ts'

const message = (id, createdAt, position = 1) => ({
  id,
  createdAt,
  position,
  conversationId: 'room',
  senderId: 'peer',
  kind: 'message',
  text: '',
  reactions: [],
})
const page = {
  items: [
    message('a', '2026-11-14T12:00:00.000000001Z', 3),
    message('b', '2026-11-14T12:00:00.000000002Z', 1),
    { ...message('pending', '2026-11-14T13:00:00Z'), delivery: 'pending' },
  ],
  nextCursor: null,
}
assert.equal(observedReadBoundary(page, 'room'), 'b', 'preserve sub-millisecond time order')
assert.equal(observedReadBoundary({ ...page, partial: true }, 'room'), undefined)
assert.equal(observedReadBoundary({ ...page, detached: true }, 'room'), undefined)
assert.equal(observedReadBoundary({ ...page, hasNewer: true }, 'room'), undefined)
assert.equal(observedReadBoundary({ ...page, offline: true }, 'room'), undefined)
assert.equal(observedReadBoundary({ ...page, items: [] }, 'room'), undefined)
assert.equal(observedReadBoundary({ ...page, items: [message('a', 'invalid')] }, 'room'), undefined)
assert.equal(
  observedReadBoundary({ ...page, items: [message('a', '2026-11-14T12:00:00Z', 2 ** 54)] }, 'room'),
  undefined,
)
assert.equal(
  observedReadBoundary(
    {
      ...page,
      items: [message('a', '2026-11-14T12:00:00Z', 2), message('b', '2026-11-14T12:00:00Z', 2)],
    },
    'room',
  ),
  'b',
)

globalThis.location = new URL('https://example.test/channel/room')
const schema = {
  paths: {
    '/api/v1/conversations/{id}': {
      patch: {
        requestBody: {
          content: {
            'application/json': { schema: { $ref: '#/components/schemas/ConversationPatchBody' } },
          },
        },
      },
    },
  },
  components: {
    schemas: {
      ConversationPatchBody: { properties: { readThroughMessageId: { type: 'string' } } },
    },
  },
}
let calls = 0
let release
const discovery = createReadThroughDiscovery('/api/v1', async () => {
  calls++
  await new Promise((resolve) => {
    release = resolve
  })
  return new Response(JSON.stringify(schema))
})
const first = discovery.supported()
const second = discovery.supported()
assert.equal(calls, 1, 'concurrent reads share one discovery')
release()
assert.equal(await first, true)
assert.equal(await second, true)
assert.equal(await discovery.supported(), true)
assert.equal(calls, 1, 'discovery is cached for the session')
discovery.reset()
const next = discovery.supported()
assert.equal(calls, 2)
release()
assert.equal(await next, true)
const legacy = createReadThroughDiscovery('/api/v1', async () => new Response('{}'))
assert.equal(await legacy.supported(), false, 'old or malformed schemas fail closed')
const unavailable = createReadThroughDiscovery('/api/v1', async () => {
  throw new Error('offline')
})
assert.equal(await unavailable.supported(), false)
const tooLarge = createReadThroughDiscovery(
  '/api/v1',
  async () => new Response('x'.repeat(2_000_001)),
)
assert.equal(await tooLarge.supported(), false, 'discovery bounds the downloaded schema')
let finishStale
const changing = createReadThroughDiscovery('/api/v1', async () => {
  await new Promise((resolve) => {
    finishStale = resolve
  })
  return new Response(JSON.stringify(schema))
})
const stale = changing.supported()
changing.reset()
finishStale()
assert.equal(await stale, false, 'old backend sessions cannot enable new sessions')
console.log('read-through boundary and discovery checks passed')
