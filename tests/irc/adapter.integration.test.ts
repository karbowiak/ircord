import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { IrcAdapter } from '../../app/irc/adapter'
import { TcpIrcTransport } from './tcpTransport'

const IRC_HOST = process.env.IRCORD_TEST_IRC_HOST || '127.0.0.1'
const IRC_PORT = Number(process.env.IRCORD_TEST_IRC_PORT || 6667)
const IRC_SERVER_NAME = process.env.IRCORD_TEST_IRC_SERVER_NAME || 'ergo.test'

const suffix = `${Date.now()}${Math.floor(Math.random() * 1000)}`
const channel = `#ircord-ulw-${suffix}`

const aliceNick = `alice${suffix}`
const bobNick = `bob${suffix}`

const alice = new IrcAdapter({
  nick: aliceNick,
  username: aliceNick,
  realname: 'Alice IRCord',
  serverName: IRC_SERVER_NAME,
  transport: new TcpIrcTransport({ host: IRC_HOST, port: IRC_PORT }),
})

const bob = new IrcAdapter({
  nick: bobNick,
  username: bobNick,
  realname: 'Bob IRCord',
  serverName: IRC_SERVER_NAME,
  transport: new TcpIrcTransport({ host: IRC_HOST, port: IRC_PORT }),
})

let rootMsgId: string | null = null
let replyMsgId: string | null = null

const aliceEvents: ReturnType<typeof collectEvents> = []
const bobEvents: ReturnType<typeof collectEvents> = []

describe('IRC adapter integration (Ergo localhost)', () => {
  beforeAll(async () => {
    await alice.connect()
    await bob.connect()

    alice.onMessage(event => {
      aliceEvents.push(event)
    })
    bob.onMessage(event => {
      bobEvents.push(event)
    })

    await alice.createChannel(channel)
    await bob.joinChannel(channel)
  }, 20000)

  afterAll(() => {
    alice.disconnect()
    bob.disconnect()
  })

  it('connects and sends messages observable by peers', async () => {
    rootMsgId = await alice.sendMessage(channel, 'root message from alice')
    expect(rootMsgId).toBeTruthy()

    const seenByBob = await waitForEvent(
      bobEvents,
      event => event.author === aliceNick && event.content === 'root message from alice' && event.channelId === channel,
    )

    expect(seenByBob.msgid).toBe(rootMsgId)
  }, 30000)

  it('supports replies linked by msgid', async () => {
    expect(rootMsgId).toBeTruthy()
    replyMsgId = await bob.replyToMessage(channel, rootMsgId!, 'reply from bob')
    expect(replyMsgId).toBeTruthy()

    const seenByAlice = await waitForEvent(
      aliceEvents,
      event => event.author === bobNick
        && event.content === 'reply from bob'
        && event.replyToMsgid === rootMsgId
        && event.tags['+draft/reply'] === rootMsgId
        && event.tags['+reply'] === rootMsgId,
    )

    expect(seenByAlice.msgid).toBe(replyMsgId)
  }, 30000)

  it('supports reactions on messages', async () => {
    expect(rootMsgId).toBeTruthy()
    const reacted = await bob.addReaction(channel, rootMsgId!, '👍')
    expect(reacted).toBe(true)

    const reactionSeen = await waitForEvent(
      aliceEvents,
      event => event.author === bobNick
        && event.reaction === '👍'
        && event.replyToMsgid === rootMsgId
        && event.tags['+draft/react'] === '👍'
        && event.tags['+react'] === '👍',
    )

    expect(reactionSeen.channelId).toBe(channel)
  }, 30000)

  it('roundtrips custom draft/client-only tags', async () => {
    const customTagValue = 'one two;three\\four'
    const message = 'message with custom tags'

    const msgId = await alice.sendMessageWithTags(channel, message, {
      '+draft/custom': customTagValue,
      '+ircord/custom': customTagValue,
    })

    expect(msgId).toBeTruthy()

    const seenByBob = await waitForEvent(
      bobEvents,
      event => event.author === aliceNick
        && event.content === message
        && event.tags['+draft/custom'] === customTagValue
        && event.tags['+ircord/custom'] === customTagValue,
    )

    expect(seenByBob.msgid).toBe(msgId)
  }, 30000)

  it('supports edit and delete flows', async () => {
    expect(replyMsgId).toBeTruthy()

    const editedMsgId = await bob.editMessage(channel, replyMsgId!, 'edited reply from bob')
    expect(editedMsgId).toBeTruthy()

    const deleted = await bob.deleteMessage(channel, editedMsgId!, 'cleanup')
    if (bob.hasCapability('draft/message-redaction')) {
      expect(deleted).toBe(true)
    } else {
      expect(deleted).toBe(false)
    }
  }, 30000)

  it('supports slap with trout and kicking users', async () => {
    const slapMsgId = await alice.slapWithTrout(channel, bobNick)
    expect(slapMsgId).toBeTruthy()

    const actionSeen = await waitForEvent(
      bobEvents,
      event => event.author === aliceNick && event.content.includes(`ACTION slaps ${bobNick} around a bit with a large trout`),
    )

    expect(actionSeen.msgid).toBe(slapMsgId)

    const kicked = await alice.kickUser(channel, bobNick, 'test kick')
    expect(kicked).toBe(true)
  }, 30000)

  it('returns channel members via requestNames', async () => {
    // Rejoin bob since he was kicked in the previous test
    await bob.joinChannel(channel)

    const names = await alice.requestNames(channel)
    expect(names.length).toBeGreaterThanOrEqual(2)

    const aliceEntry = names.find(n => n.nick === aliceNick)
    expect(aliceEntry).toBeTruthy()

    const bobEntry = names.find(n => n.nick === bobNick)
    expect(bobEntry).toBeTruthy()
    expect(bobEntry!.mode).toBe('regular')
  }, 30000)

  it('fetches channel history via requestHistory', async () => {
    // Send a unique message so we can find it in history
    const historyMarker = `history-check-${Date.now()}`
    await alice.sendMessage(channel, historyMarker)
    await new Promise(r => setTimeout(r, 200))

    const history = await alice.requestHistory(channel, 50)

    if (alice.hasCapability('draft/chathistory')) {
      expect(history.length).toBeGreaterThan(0)
      const found = history.find(m => m.content === historyMarker)
      expect(found).toBeTruthy()
      expect(found!.author).toBe(aliceNick)
    } else {
      // If server doesn't support chathistory, should return empty
      expect(history).toEqual([])
    }
  }, 30000)

  it('measures latency via PING/PONG', async () => {
    alice.measureLatency()
    // Wait for the PONG to come back
    await new Promise(r => setTimeout(r, 1000))
    expect(alice.latencyMs).not.toBeNull()
    expect(alice.latencyMs!).toBeGreaterThanOrEqual(0)
    expect(alice.latencyMs!).toBeLessThan(5000)
  }, 30000)

  it('captures raw log entries', () => {
    const log = alice.rawLog
    expect(log.length).toBeGreaterThan(0)

    const hasOutgoing = log.some(e => e.direction === 'out')
    const hasIncoming = log.some(e => e.direction === 'in')
    expect(hasOutgoing).toBe(true)
    expect(hasIncoming).toBe(true)

    // Each entry should have time and line
    for (const entry of log) {
      expect(entry.time).toBeTruthy()
      expect(entry.line).toBeTruthy()
    }
  })
})

function collectEvents() {
  return [] as Array<{
    channelId: string
    author: string
    content: string
    msgid: string | null
    replyToMsgid: string | null
    reaction: string | null
    tags: Record<string, string>
    timestamp: string
  }>
}

function waitForEvent(
  events: ReturnType<typeof collectEvents>,
  matcher: (event: ReturnType<typeof collectEvents>[number]) => boolean,
  timeoutMs = 5000,
) {
  return new Promise<ReturnType<typeof collectEvents>[number]>((resolve, reject) => {
    const startedAt = Date.now()

    const tick = () => {
      const match = events.find(matcher)
      if (match) {
        resolve(match)
        return
      }

      if (Date.now() - startedAt > timeoutMs) {
        reject(new Error(`Timed out waiting for IRC event after ${timeoutMs}ms`))
        return
      }

      setTimeout(tick, 25)
    }

    tick()
  })
}
