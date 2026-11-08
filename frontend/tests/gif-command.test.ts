import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createGifCommand, gifUsage, pickRandom } from '../shared/utils/gif-command'
import type { GifItem } from '../shared/types/media'
import type { Message } from '../shared/types/chat'

// Lookups are held behind a gate; a timeout makes a missing guard fail instead of hanging.
const gated = { timeout: 2000 }

const gif = (slug: string) =>
  ({ slug, url: `https://klipy.com/gifs/${slug}` }) as unknown as GifItem
const fixtures = [gif('one'), gif('two'), gif('three')]

function harness(options: { items?: GifItem[]; error?: Error; send?: boolean; random?: number } = {}) {
  const log = { searches: [] as string[], sent: [] as [string, string, string?][], shared: [] as [string, string][], notices: [] as string[] }
  let release: () => void = () => {}
  const gate = new Promise<void>((resolve) => (release = resolve))
  const command = createGifCommand({
    search: async (query) => {
      log.searches.push(query)
      await gate
      if (options.error) throw options.error
      return options.items ?? fixtures
    },
    send: ({ conversationId, replyTo }, item) => {
      if (options.send === false) return false
      log.sent.push(replyTo ? [conversationId, item.slug, replyTo.id] : [conversationId, item.slug])
      return true
    },
    share: (item, query) => log.shared.push([item.slug, query]),
    notify: (text) => log.notices.push(text),
    random: () => options.random ?? 0,
  })
  return { command, log, release }
}

test('pickRandom covers the whole range and handles empty lists', () => {
  assert.equal(pickRandom([], () => 0), undefined)
  assert.equal(pickRandom(fixtures, () => 0)?.slug, 'one')
  assert.equal(pickRandom(fixtures, () => 0.5)?.slug, 'two')
  assert.equal(pickRandom(fixtures, () => 0.999)?.slug, 'three')
  assert.equal(pickRandom(fixtures, () => 1)?.slug, 'three')
})

test('an empty query shows usage and searches nothing', gated, async () => {
  const { command, log } = harness()
  assert.equal(await command.run('   ', { conversationId: 'c1' }), 'kept')
  assert.deepEqual(log.notices, [gifUsage])
  assert.deepEqual(log.searches, [])
})

test('the full multi-word query is searched and a random match is sent and shared', gated, async () => {
  const { command, log, release } = harness({ random: 0.7 })
  const pending = command.run('  dancing   cat  ', { conversationId: 'c1' })
  release()
  assert.equal(await pending, 'sent')
  assert.deepEqual(log.searches, ['dancing   cat'])
  assert.deepEqual(log.sent, [['c1', 'three']])
  assert.deepEqual(log.shared, [['three', 'dancing   cat']])
  assert.deepEqual(log.notices, [])
})

test('no matches keep the input with a message', async () => {
  const { command, log, release } = harness({ items: [] })
  release()
  assert.equal(await command.run('nothinghere', { conversationId: 'c1' }), 'kept')
  assert.deepEqual(log.sent, [])
  assert.match(log.notices[0]!, /No GIFs matched “nothinghere”/)
})

test('provider errors (rate limits, missing key) are surfaced and the input kept', async () => {
  for (const message of ['GIF search is busy. Try again in a moment.', 'GIF search is not configured on this instance.']) {
    const { command, log, release } = harness({ error: new Error(message) })
    release()
    assert.equal(await command.run('cat', { conversationId: 'c1' }), 'kept')
    assert.deepEqual(log.notices, [message])
    assert.deepEqual(log.sent, [])
  }
})

test('a refused send keeps the input and does not register a share', async () => {
  const { command, log, release } = harness({ send: false })
  release()
  assert.equal(await command.run('cat', { conversationId: 'c1' }), 'kept')
  assert.deepEqual(log.shared, [])
  assert.deepEqual(log.notices, ['Reconnect before sending this GIF.'])
})

test('repeated submission while a lookup is in flight sends only once', gated, async () => {
  const { command, log, release } = harness()
  const first = command.run('cat', { conversationId: 'c1' })
  assert.equal(command.lookup('c1'), 'cat')
  assert.equal(await command.run('cat', { conversationId: 'c1' }), 'kept')
  assert.deepEqual(log.notices, ['Still looking for a GIF…'])
  release()
  assert.equal(await first, 'sent')
  assert.equal(command.lookup('c1'), undefined)
  assert.equal(log.searches.length, 1)
  assert.equal(log.sent.length, 1)
})

test('the GIF goes to the target fixed at submit time, including its reply', gated, async () => {
  // Capturing the open conversation before the lookup is useCommands' job; the browser test
  // covers navigating away. Here: the controller sends to exactly the target it was given.
  const { command, log, release } = harness()
  const replyTo = { id: 'm-42' } as Message
  const pending = command.run('cat', { conversationId: 'channel-a', replyTo })
  assert.equal(command.lookup('direct-b'), undefined)
  release()
  assert.equal(await pending, 'sent')
  assert.deepEqual(log.sent, [['channel-a', 'one', 'm-42']])
})
