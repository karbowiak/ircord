import { randomUUID } from 'node:crypto'
import { wireConversation } from '../../shared/utils/conversation'
import type {
  Bootstrap,
  Conversation,
  DurableEvent,
  Message,
  MessagePage,
  Network,
  NetworkInput,
  ServerLine,
  Upload,
  ServerEvent,
  ChannelListEntry,
} from '../../shared/types/chat'
import { createFixtures } from './fixtures'
import {
  attachmentFor,
  membershipEvent,
  networkSettings,
  seedServerLog,
  storeSecret,
} from './irc-mock'
import { validGif } from '../../shared/utils/media'
import type { GifItem } from '../../shared/types/media'
import { mentionsNick } from '../../shared/utils/identity'
import { stripFormatting } from '../../shared/utils/irc'
import { notifyIncoming } from './push'

export const limits = {
  sessions: 500,
  messages: 300,
  events: 200,
  text: 4000,
  connections: 8,
  conversations: 100,
  networks: 20,
  serverLines: 500,
  uploads: 20,
  uploadBytes: 8 * 1024 * 1024,
}
type Listener = (event: ServerEvent) => void
export interface DemoSession {
  id: string
  touched: number
  bootstrap: Bootstrap
  messages: Record<string, Message[]>
  events: DurableEvent[]
  listeners: Set<Listener>
  sent: Map<string, Message>
  timers: Set<ReturnType<typeof setTimeout>>
  windowStart: number
  windowMessages: number
  profileAvatar?: { bytes: Uint8Array; contentType: string; revision: string }
  /** Status window lines per network (bounded). */
  serverLogs: Record<string, ServerLine[]>
  /** Write-only network passwords; never returned to the client. */
  secrets: Record<string, string>
  uploads: Map<string, StoredUpload>
  /** Channel list modes (b, e, I, q) per conversation. */
  channelLists: Record<string, Record<string, ChannelListEntry[]>>
}
export interface StoredUpload extends Upload {
  bytes: Uint8Array
}
const sessions = new Map<string, DemoSession>()

export function createSession(id: string = randomUUID()): DemoSession {
  pruneSessions()
  if (sessions.size >= limits.sessions)
    throw new Error('The demo is busy. Please try again shortly.')
  const fixture = createFixtures()
  const session: DemoSession = {
    id,
    touched: Date.now(),
    ...fixture,
    events: [],
    listeners: new Set(),
    sent: new Map(),
    timers: new Set(),
    windowStart: Date.now(),
    windowMessages: 0,
    serverLogs: {},
    secrets: {},
    uploads: new Map(),
    channelLists: {
      general: {
        b: [{ mask: '*!*@spam.example', setBy: 'mika', setAt: '2026-10-01T12:00:00.000Z' }],
        q: [{ mask: 'loudbot!*@*', setBy: 'mika', setAt: '2026-10-20T09:30:00.000Z' }],
      },
    },
  }
  for (const network of session.bootstrap.networks) seedServerLog(session, network)
  sessions.set(id, session)
  return session
}
export function findSession(id: string | undefined) {
  if (!id) return undefined
  const session = sessions.get(id)
  if (session && Date.now() - session.touched < 60 * 60 * 1000) {
    session.touched = Date.now()
    return session
  }
  if (session) disposeSession(session)
}
function disposeSession(session: DemoSession) {
  for (const listener of session.listeners)
    listener({
      v: 1,
      type: 'session.resync',
      reason: 'Your demo session expired. Reload to start again.',
    })
  session.timers.forEach(clearTimeout)
  session.listeners.clear()
  sessions.delete(session.id)
}
export function pruneSessions() {
  for (const session of sessions.values())
    if (Date.now() - session.touched > 60 * 60 * 1000) disposeSession(session)
}

type EventInput = DurableEvent extends infer E
  ? E extends DurableEvent
    ? Omit<E, 'v' | 'seq'>
    : never
  : never
export function emit(session: DemoSession, input: EventInput) {
  const event = { v: 1, seq: ++session.bootstrap.cursor, ...structuredClone(input) } as DurableEvent
  session.events.push(event)
  if (session.events.length > limits.events) session.events.shift()
  for (const listener of session.listeners) listener(event)
  return event
}
export function getConversation(session: DemoSession, id: string): Conversation {
  const conversation = session.bootstrap.conversations.find((c) => c.id === id)
  if (!conversation) throw new Error('Conversation not found.')
  return conversation
}
export function history(
  session: DemoSession,
  conversationId: string,
  before?: string,
  limit = 40,
): MessagePage {
  getConversation(session, conversationId)
  const messages = session.messages[conversationId] ?? []
  let end = before ? messages.findIndex((m) => m.id === before) : messages.length
  if (end < 0) throw new Error('This history cursor has expired. Reload the conversation.')
  const start = Math.max(0, end - Math.min(100, Math.max(1, limit)))
  return { items: messages.slice(start, end), nextCursor: start > 0 ? messages[start]!.id : null }
}
export function appendMessage(session: DemoSession, message: Message) {
  const messages = (session.messages[message.conversationId] ??= [])
  messages.push(message)
  if (messages.length > limits.messages) messages.splice(0, messages.length - limits.messages)
  // Unread and mention counts live on the server so they survive reloads. Clients count the
  // same events locally, so no conversation.updated is sent for these increments.
  const c = session.bootstrap.conversations.find((x) => x.id === message.conversationId)
  if (c && message.senderId !== 'you' && message.kind !== 'event') {
    c.unreadCount++
    const nick = session.bootstrap.networks.find((n) => n.id === c.networkId)?.nick
    if (c.kind === 'channel' && mentionsNick(stripFormatting(message.text), nick)) c.mentionCount++
  }
  emit(session, { type: 'message.created', data: message })
  if (message.senderId !== 'you') void notifyIncoming(session, message).catch(() => {})
}
export function sendMessage(
  session: DemoSession,
  input: {
    clientId: string
    conversationId: string
    text: string
    replyToId?: string
    gif?: GifItem
    kind?: 'message' | 'action'
    attachments?: string[]
  },
): Message {
  const existing = session.sent.get(input.clientId)
  if (existing) return existing
  const conversation = getConversation(session, input.conversationId)
  if (input.gif && (!validGif(input.gif) || input.text !== input.gif.url))
    throw new Error('This GIF could not be sent. Choose it again from the picker.')
  const network = session.bootstrap.networks.find((n) => n.id === conversation.networkId)
  if (network?.status !== 'connected')
    throw new Error('This network is disconnected. Reconnect it in settings.')
  const upload = input.attachments?.length ? session.uploads.get(input.attachments[0]!) : undefined
  if (input.attachments?.length && !upload)
    throw new Error('That upload has expired. Attach it again.')
  const kind = input.kind === 'action' ? 'action' : 'message'
  const text = typeof input.text === 'string' ? input.text.trim() : ''
  if (!text || text.length > limits.text)
    throw new Error(`Messages must contain between 1 and ${limits.text} characters.`)
  if (!/^[\w-]{1,100}$/.test(input.clientId)) throw new Error('Invalid message identifier.')
  if (Date.now() - session.windowStart > 10_000) {
    session.windowStart = Date.now()
    session.windowMessages = 0
  }
  if (++session.windowMessages > 30)
    throw new Error('You’re sending messages quickly. Try again in a few seconds.')
  const parent = input.replyToId
    ? session.messages[conversation.id]?.find((m) => m.id === input.replyToId)
    : undefined
  const message: Message = {
    id: randomUUID(),
    clientId: input.clientId,
    conversationId: conversation.id,
    senderId: 'you',
    text,
    createdAt: new Date().toISOString(),
    kind,
    ...(input.gif ? { gif: input.gif } : {}),
    ...(upload ? { attachment: attachmentFor(upload) } : {}),
    reactions: [],
    ...(parent
      ? {
          replyTo: {
            id: parent.id,
            nick:
              session.bootstrap.members.find((m) => m.id === parent.senderId)?.nick ?? 'unknown',
            text: parent.text.slice(0, 160),
          },
        }
      : {}),
  }
  session.sent.set(input.clientId, message)
  if (session.sent.size > 300) session.sent.delete(session.sent.keys().next().value!)
  appendMessage(session, message)
  if (conversation.previewMode === 'server') mockPreview(session, message)
  // The echo bot “types” before replying. Typing events are ephemeral: no seq, no replay.
  for (const listener of session.listeners)
    listener({
      v: 1,
      type: 'typing',
      data: { conversationId: conversation.id, memberId: 'echo', state: 'active' },
    })
  const timer = setTimeout(() => {
    session.timers.delete(timer)
    appendMessage(session, {
      id: randomUUID(),
      conversationId: conversation.id,
      senderId: 'echo',
      text,
      createdAt: new Date().toISOString(),
      kind,
      ...(input.gif ? { gif: input.gif } : {}),
      ...(upload ? { attachment: attachmentFor(upload) } : {}),
      reactions: [],
      replyTo: { id: message.id, nick: session.bootstrap.user.nick, text: text.slice(0, 160) },
    })
  }, 550)
  timer.unref?.()
  session.timers.add(timer)
  return message
}
export function toggleReaction(
  session: DemoSession,
  conversationId: string,
  messageId: string,
  emoji: string,
) {
  getConversation(session, conversationId)
  if (
    typeof emoji !== 'string' ||
    !emoji ||
    emoji.length > 16 ||
    !/\p{Extended_Pictographic}/u.test(emoji)
  )
    throw new Error('Choose a valid emoji.')
  const message = session.messages[conversationId]?.find((m) => m.id === messageId)
  if (!message) throw new Error('Message not found.')
  const reaction = message.reactions.find((r) => r.emoji === emoji)
  if (reaction)
    reaction.memberIds = reaction.memberIds.includes('you')
      ? reaction.memberIds.filter((id) => id !== 'you')
      : [...reaction.memberIds, 'you']
  else {
    if (message.reactions.length >= 20)
      throw new Error('This message has reached the reaction limit.')
    message.reactions.push({ emoji, memberIds: ['you'] })
  }
  message.reactions = message.reactions.filter((r) => r.memberIds.length)
  emit(session, { type: 'message.updated', data: message })
  return message
}
/** Stands in for the backend's protected fetch: inert metadata derived from the URL. */
function mockPreview(session: DemoSession, message: Message) {
  const url = /https:\/\/[^\s<>"]+/.exec(message.text)?.[0]
  if (!url) return
  const host = new URL(url).hostname.replace(/^www\./, '')
  // Share pages of the supported GIF providers resolve to their GIF, as the backend does.
  const provider = /(^|\.)(tenor|giphy)\.com$/.exec(host)?.[2]
  const label = provider === 'tenor' ? 'Tenor' : 'GIPHY'
  const timer = setTimeout(() => {
    session.timers.delete(timer)
    const updated: Message = {
      ...message,
      preview: provider
        ? {
            url,
            title: `${label} GIF`,
            siteName: label,
            media: {
              url: `https://media.${provider}.com/demo/ircord.gif`,
              kind: 'gif',
              width: 220,
              height: 160,
            },
          }
        : {
            url,
            title: `${host} · ${new URL(url).pathname.split('/').filter(Boolean).at(-1) ?? 'Home'}`,
            description: `A page on ${host}, previewed by the server.`,
            siteName: host,
          },
      previewSource: 'server',
    }
    const list = session.messages[message.conversationId]
    const index = list?.findIndex((m) => m.id === message.id) ?? -1
    if (list && index >= 0) list[index] = updated
    emit(session, { type: 'message.updated', data: updated })
  }, 300)
  timer.unref?.()
  session.timers.add(timer)
}

export function patchConversation(
  session: DemoSession,
  id: string,
  input: { read?: boolean; favorite?: boolean; previewMode?: string },
) {
  const c = getConversation(session, id)
  if (input.previewMode === 'server' || input.previewMode === 'client')
    c.previewMode = input.previewMode
  if (input.read) {
    c.unreadCount = 0
    c.mentionCount = 0
  }
  if (typeof input.favorite === 'boolean') c.favorite = input.favorite
  emit(session, { type: 'conversation.updated', data: wireConversation(c) })
  return c
}
export function addNetwork(session: DemoSession, input: NetworkInput) {
  if (session.bootstrap.networks.length >= limits.networks)
    throw new Error('The demo supports up to 20 networks.')
  const settings = networkSettings(input)
  const network: Network = {
    id: randomUUID(),
    initials: settings.name.slice(0, 2).toUpperCase(),
    color: 'lavender',
    status: 'connected',
    capabilities: ['replies', 'reactions', 'history', 'profiles'],
    isupport: {
      NETWORK: settings.name,
      PREFIX: '(qaohv)~&@%+',
      CHANMODES: 'beIq,k,fjl,CPQSTcgimnprstuz',
      CHANTYPES: '#',
      CASEMAPPING: 'rfc1459',
    },
    features: { setname: true, metadata: [], away: true },
    ...settings,
  }
  storeSecret(session, network.id, input)
  session.bootstrap.networks.push(network)
  seedServerLog(session, network)
  emit(session, { type: 'network.updated', data: network })
  return network
}
export function updateNetwork(session: DemoSession, id: string, input: NetworkInput) {
  const network = getNetwork(session, id)
  Object.assign(network, networkSettings(input, network))
  storeSecret(session, id, input)
  emit(session, { type: 'network.updated', data: network })
  return network
}
export function removeNetwork(session: DemoSession, id: string) {
  getNetwork(session, id)
  session.bootstrap.networks = session.bootstrap.networks.filter((n) => n.id !== id)
  for (const c of session.bootstrap.conversations.filter((c) => c.networkId === id))
    delete session.messages[c.id]
  session.bootstrap.conversations = session.bootstrap.conversations.filter(
    (c) => c.networkId !== id,
  )
  delete session.serverLogs[id]
  delete session.secrets[id]
  emit(session, { type: 'network.removed', data: { id } })
}
export function getNetwork(session: DemoSession, id: string) {
  const network = session.bootstrap.networks.find((n) => n.id === id)
  if (!network) throw new Error('Network not found.')
  return network
}
export function addConversation(
  session: DemoSession,
  input: { networkId: string; name?: string; memberId?: string },
) {
  if (!session.bootstrap.networks.some((n) => n.id === input.networkId))
    throw new Error('Network not found.')
  const member = session.bootstrap.members.find((m) => m.id === input.memberId)
  const name = member?.nick ?? input.name?.trim().replace(/^#/, '')
  if (!name || !/^[\w-]{1,50}$/.test(name))
    throw new Error('Use a channel name with letters, numbers, underscores or hyphens.')
  const kind = member ? 'direct' : 'channel'
  const existing = session.bootstrap.conversations.find(
    (c) => c.networkId === input.networkId && c.name === name && c.kind === kind,
  )
  if (existing) return existing
  if (session.bootstrap.conversations.length >= limits.conversations)
    throw new Error('The demo supports up to 100 conversations.')
  const c: Conversation = {
    id: randomUUID(),
    name,
    kind,
    networkId: input.networkId,
    topic: member
      ? `Your conversation with ${member.displayName}.`
      : 'A fresh start. Say something good.',
    unreadCount: 0,
    mentionCount: 0,
    favorite: false,
    memberIds: member ? ['you', member.id, 'echo'] : ['you', 'echo'],
    ...(member ? {} : { memberModes: { you: '@' }, modes: 'nt' }),
  }
  session.bootstrap.conversations.push(c)
  session.messages[c.id] = []
  emit(session, { type: 'conversation.updated', data: wireConversation(c) })
  if (!member) membershipEvent(session, c, { type: 'join', actorId: 'you' })
  return c
}
