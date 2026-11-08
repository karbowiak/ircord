import { test } from 'node:test'
import assert from 'node:assert/strict'
import { matchCommands, type CommandSpec } from '../shared/utils/commands'
import {
  ATTRACT_MS,
  attractClock,
  offscreen,
  resolveAttract,
  rowVisible,
} from '../shared/utils/attract'
import { parseServerEvent } from '../app/utils/transport'

const commands: CommandSpec[] = [
  { name: 'away', args: '', description: '' },
  { name: 'part', args: '', description: '', channel: true },
  { name: 'attract', args: '', description: '', channel: true, operator: true, hidden: true },
]
const names = (query: string, where = { channel: true, operator: true }) =>
  matchCommands(commands, query, where).map((c) => c.name)

test('/attract stays out of the menu until /attr, and only for operators in a channel', () => {
  assert.deepEqual(names(''), ['away', 'part'])
  for (const query of ['a', 'at', 'att']) assert.ok(!names(query).includes('attract'), query)
  assert.deepEqual(names('attr'), ['attract'])
  assert.deepEqual(names('ATTRACT'), ['attract'])
  assert.deepEqual(names('attr', { channel: true, operator: false }), [])
  assert.deepEqual(names('attr', { channel: false, operator: true }), [])
  assert.deepEqual(names('a', { channel: false, operator: false }), ['away'])
})

test('attract targets the message row and the avatar at the head of its group', () => {
  const rows = [
    { id: 'm1', senderId: 'ana' },
    undefined,
    { id: 'm2', senderId: 'bo' },
    { id: 'm3', senderId: 'bo' },
    { id: 'm4', senderId: 'bo' },
  ]
  const grouped = (i: number) => i === 3 || i === 4
  assert.deepEqual(resolveAttract(rows, { messageId: 'm4', memberId: 'bo' }, grouped), {
    row: 4,
    avatar: 2,
  })
  assert.deepEqual(resolveAttract(rows, { messageId: 'm1', memberId: 'ana' }, grouped), {
    row: 0,
    avatar: 0,
  })
  // Not loaded: the sender's newest loaded message stands in; nobody's, nothing plays.
  assert.deepEqual(resolveAttract(rows, { messageId: 'old', memberId: 'bo' }, grouped), {
    row: 4,
    avatar: 2,
  })
  assert.equal(resolveAttract(rows, { messageId: 'old', memberId: 'cy' }, grouped), undefined)
})

test('attract visibility and duration', () => {
  const view = { scrollTop: 100, clientHeight: 200 }
  assert.ok(rowVisible(90, 20, view))
  assert.ok(rowVisible(290, 40, view))
  assert.ok(!rowVisible(40, 60, view))
  assert.ok(!rowVisible(300, 40, view))
  assert.equal(ATTRACT_MS, 3000)
  // Off-screen above gets the top-edge cue, below gets the pill.
  assert.equal(offscreen(5, { start: 150, size: 20 }, 0, view), undefined)
  assert.equal(offscreen(5, { start: 40, size: 20 }, 0, view), 'above')
  assert.equal(offscreen(5, { start: 400, size: 20 }, 0, view), 'below')
  assert.equal(offscreen(2, undefined, 10, view), 'above')
  assert.equal(offscreen(30, undefined, 10, view), 'below')
})

test('the effect clock places late-mounted animations in step and carries the duration', () => {
  assert.deepEqual(attractClock(1000, 1800), {
    '--attract-delay': '-800ms',
    '--attract-ms': '3000ms',
  })
  assert.deepEqual(attractClock(1000, 1000)['--attract-delay'], '0ms')
})

test('the WebSocket contract accepts an unsequenced attract event', () => {
  const event = {
    v: 1,
    type: 'attract',
    data: { conversationId: 'general', memberId: 'ana', messageId: 'm1' },
  }
  assert.equal(parseServerEvent(JSON.stringify(event)).type, 'attract')
  assert.throws(() => parseServerEvent(JSON.stringify({ ...event, data: { conversationId: 'x' } })))
})
