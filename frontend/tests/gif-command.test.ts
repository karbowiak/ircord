import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createGifCommand, gifUsage, pickRandom } from '../shared/utils/gif-command'
import type { GifItem } from '../shared/types/media'

const gif = (slug: string) =>
  ({ slug, url: `https://klipy.com/gifs/${slug}` }) as unknown as GifItem
const fixtures = [gif('one'), gif('two'), gif('three')]

function harness(options: { items?: GifItem[]; error?: Error; send?: boolean; random?: number } = {}) {
  const log = { searches: [] as string[], sent: [] as [string, string][], shared: [] as [string, string][], notices: [] as string[] }
  let release: () => void = () => {}
  const gate = new Promise<void>((resolve) => (release = resolve))
  const command = createGifCommand({
    search: async (query) => {
      log.searches.push(query)
      await gate
      if (options.error) throw options.error
      return options.items ?? fixtures
    },
    send: (conversationId, item) => {
      if (options.send === false) return false
      log.sent.push([conversationId, item.slug])
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

test('an empty query shows usage and searches nothing', async () => {
  const { command, log } = harness()
  assert.equal(await command.run('   ', 'c1'), 'kept')
  assert.deepEqual(log.notices, [gifUsage])
  assert.deepEqual(log.searches, [])
})

test('the full multi-word query is searched and a random match is sent and shared', async () => {
  const { command, log, release } = harness({ random: 0.7 })
  const pending = command.run('  dancing   cat  ', 'c1')
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
  assert.equal(await command.run('nothinghere', 'c1'), 'kept')
  assert.deepEqual(log.sent, [])
  assert.match(log.notices[0]!, /No GIFs matched “nothinghere”/)
})

test('provider errors (rate limits, missing key) are surfaced and the input kept', async () => {
  for (const message of ['GIF search is busy. Try again in a moment.', 'GIF search is not configured on this instance.']) {
    const { command, log, release } = harness({ error: new Error(message) })
    release()
    assert.equal(await command.run('cat', 'c1'), 'kept')
    assert.deepEqual(log.notices, [message])
    assert.deepEqual(log.sent, [])
  }
})

test('a refused send keeps the input and does not register a share', async () => {
  const { command, log, release } = harness({ send: false })
  release()
  assert.equal(await command.run('cat', 'c1'), 'kept')
  assert.deepEqual(log.shared, [])
  assert.deepEqual(log.notices, ['Reconnect before sending this GIF.'])
})

test('repeated submission while a lookup is in flight sends only once', async () => {
  const { command, log, release } = harness()
  const first = command.run('cat', 'c1')
  assert.equal(command.isBusy('c1'), true)
  assert.equal(await command.run('cat', 'c1'), 'kept')
  assert.deepEqual(log.notices, ['Still looking for a GIF…'])
  release()
  assert.equal(await first, 'sent')
  assert.equal(command.isBusy('c1'), false)
  assert.equal(log.searches.length, 1)
  assert.equal(log.sent.length, 1)
})

test('the result goes to the conversation the command was submitted in', async () => {
  const { command, log, release } = harness()
  let active = 'channel-a'
  const pending = command.run('cat', active)
  active = 'direct-b' // the user navigates away while the search runs
  assert.equal(command.isBusy(active), false)
  release()
  assert.equal(await pending, 'sent')
  assert.deepEqual(log.sent, [['channel-a', 'one']])
})
