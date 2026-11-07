import { randomUUID } from 'node:crypto'
import type { Conversation, Member } from '../../shared/types/chat'
import { wireConversation } from '../../shared/utils/conversation'
import { appendMessage, emit, type DemoSession } from './demo-store'
import { membershipEvent } from './irc-mock'

/**
 * A repeatable busy channel for performance work (#13): a large roster, deep history, and a
 * sustained flood of messages, joins, parts and presence changes. Demo mode only.
 */
export interface BusyLoad {
  /** People in the channel. */
  members: number
  /** Messages of history before the flood starts. */
  history: number
  /** Incoming messages per second during the flood. */
  rate: number
  /** Joins, parts and presence changes per second during the flood. */
  churn: number
  /** How long the flood lasts, in seconds. */
  seconds: number
}

export const busyLimits: BusyLoad = {
  members: 10_000,
  history: 20_000,
  rate: 100,
  churn: 50,
  seconds: 300,
}
export const busyId = 'busy'

const colors = ['lavender', 'mint', 'peach', 'blue', 'rose', 'yellow'] as const
const lines = [
  'anyone tried the new release yet?',
  'that build failed again, looking into it',
  'lol',
  'https://example.com/changelog',
  'brb coffee ☕',
  '\x02heads up:\x02 maintenance at 20:00 UTC',
  'I think the issue is in the parser, not the lexer',
  'thanks!',
  'has anyone seen \x0304red\x03 text render weird on mobile?',
  'ok that makes sense now',
  'karbowiak: can you take a look at this?',
  'we should write that down somewhere 📝',
]

/** Small seeded generator, so every run produces the same channel and traffic. */
function random(seed: number) {
  let state = seed >>> 0
  return () => {
    state = (state + 0x6d2b79f5) >>> 0
    let t = state
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

export function clampLoad(input: Partial<Record<keyof BusyLoad, unknown>>): BusyLoad {
  const pick = (key: keyof BusyLoad, fallback: number) => {
    const value = Number(input[key] ?? fallback)
    return Number.isFinite(value)
      ? Math.max(0, Math.min(busyLimits[key], Math.round(value)))
      : fallback
  }
  return {
    members: pick('members', 3000),
    history: pick('history', 5000),
    rate: pick('rate', 20),
    churn: pick('churn', 5),
    seconds: pick('seconds', 60),
  }
}

/** Creates (or recreates) #busy in the first network and starts the flood. */
export function startBusyLoad(session: DemoSession, load: BusyLoad) {
  stopBusyLoad(session)
  const next = random(13)
  const network = session.bootstrap.networks[0]
  if (!network) throw new Error('Add a network first.')

  const people: Member[] = Array.from({ length: load.members }, (_, i) => ({
    id: `busy-${i}`,
    nick: `user${String(i).padStart(5, '0')}`,
    displayName: `User ${i}`,
    color: colors[i % colors.length]!,
    status: next() < 0.8 ? 'online' : 'away',
    role: 'member',
    account: '',
    bio: '',
  }))
  session.bootstrap.members = [
    ...session.bootstrap.members.filter((m) => !m.id.startsWith('busy-')),
    ...people,
  ]
  const memberModes: Record<string, string> = { you: '@' }
  for (let i = 0; i < people.length; i += 97) memberModes[people[i]!.id] = i % 2 ? '+' : '@'
  const channel: Conversation = {
    id: busyId,
    networkId: network.id,
    kind: 'channel',
    name: 'busy',
    topic: `Load test: ${load.members} people, ${load.rate} messages/s`,
    unreadCount: 0,
    mentionCount: 0,
    favorite: false,
    joined: true,
    memberIds: ['you', ...people.map((m) => m.id)],
    memberModes,
    modes: 'nt',
  }
  session.bootstrap.conversations = [
    ...session.bootstrap.conversations.filter((c) => c.id !== busyId),
    channel,
  ]

  // History, oldest first, ending a minute ago.
  const start = Date.now() - 60_000 - load.history * 2000
  session.historyLimits = {
    ...session.historyLimits,
    [busyId]: Math.max(300, load.history + load.rate * load.seconds),
  }
  session.messages[busyId] = Array.from({ length: load.history }, (_, i) => ({
    id: randomUUID(),
    conversationId: busyId,
    senderId: people[Math.floor(next() * people.length)]?.id ?? 'you',
    text: lines[Math.floor(next() * lines.length)]!,
    createdAt: new Date(start + i * 2000).toISOString(),
    kind: 'message' as const,
    reactions: [],
  }))
  // Open clients reload the snapshot, which now has the channel and its roster.
  for (const listener of session.listeners)
    listener({ v: 1, type: 'session.resync', reason: 'Busy channel created.' })

  // The flood: 10 ticks a second, so rates spread out instead of arriving in one burst.
  const tick = 100
  const until = Date.now() + load.seconds * 1000
  let messageDebt = 0
  let churnDebt = 0
  const timer = setInterval(() => {
    if (Date.now() >= until || !session.bootstrap.conversations.some((c) => c.id === busyId)) {
      stopBusyLoad(session)
      return
    }
    messageDebt += load.rate / (1000 / tick)
    churnDebt += load.churn / (1000 / tick)
    for (; messageDebt >= 1; messageDebt--) {
      appendMessage(session, {
        id: randomUUID(),
        conversationId: busyId,
        senderId: people[Math.floor(next() * people.length)]?.id ?? 'you',
        text: lines[Math.floor(next() * lines.length)]!,
        createdAt: new Date().toISOString(),
        kind: 'message',
        reactions: [],
      })
    }
    for (; churnDebt >= 1; churnDebt--) churn(session, channel, people, next)
  }, tick)
  timer.unref?.()
  session.timers.add(timer)
  session.busyTimer = timer
}

/** One join, part or presence change, as the real backend would report it. */
function churn(session: DemoSession, channel: Conversation, people: Member[], next: () => number) {
  const person = people[Math.floor(next() * people.length)]
  if (!person) return
  const roll = next()
  if (roll < 0.6) {
    person.status = person.status === 'online' ? 'away' : 'online'
    emit(session, { type: 'member.updated', data: { ...person } })
    return
  }
  const inChannel = channel.memberIds.includes(person.id)
  channel.memberIds = inChannel
    ? channel.memberIds.filter((id) => id !== person.id)
    : [...channel.memberIds, person.id]
  membershipEvent(session, channel, {
    type: inChannel ? 'part' : 'join',
    actorId: person.id,
    ...(inChannel ? { text: 'Leaving' } : {}),
  })
  emit(session, { type: 'conversation.updated', data: wireConversation(channel) })
}

export function stopBusyLoad(session: DemoSession) {
  if (!session.busyTimer) return
  clearInterval(session.busyTimer)
  session.timers.delete(session.busyTimer)
  session.busyTimer = undefined
}
