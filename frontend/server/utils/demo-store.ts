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
  SavedItem,
  SavedPage,
  SavedRef,
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
import { demoConnectionInfo } from './connection-mock'
import {
  dropNetworkInbox,
  markInboxRead,
  recordInbox,
  seedInbox,
  type DemoInboxEntry,
} from './demo-inbox'

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
  saved: 200,
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
  /** Private bookmarks by message id, like Go's saved_messages. */
  saved: Map<string, SavedRef>
  /** The attention inbox by message id, like Go's inbox_items, and read positions for it. */
  inbox: Map<string, DemoInboxEntry>
  readAt: Record<string, string>
  /** Channel list modes (b, e, I, q) per conversation. */
  channelLists: Record<string, Record<string, ChannelListEntry[]>>
  /** Larger history caps for specific conversations (the busy-channel load test). */
  historyLimits?: Record<string, number>
  busyTimer?: ReturnType<typeof setInterval>
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
    saved: new Map(),
    inbox: new Map(),
    readAt: {},
    channelLists: {
      general: {
        b: [{ mask: '*!*@spam.example', setBy: 'mika', setAt: '2026-10-01T12:00:00.000Z' }],
        q: [{ mask: 'loudbot!*@*', setBy: 'mika', setAt: '2026-10-20T09:30:00.000Z' }],
      },
    },
  }
  for (const network of session.bootstrap.networks) {
    network.account = network.nick
    seedServerLog(session, network)
  }
  for (const c of session.bootstrap.conversations)
    if (c.kind === 'channel') c.channelSettings = { notifications: 'default' }
  seedInbox(session)
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
  if (input.type === 'network.updated')
    emit(session, {
      type: 'connection.updated',
      data: demoConnectionInfo(input.data, session.windowStart),
    })
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
  around?: string,
): MessagePage {
  getConversation(session, conversationId)
  const messages = session.messages[conversationId] ?? []
  const size = Math.min(100, Math.max(1, limit))
  if (around) {
    const index = messages.findIndex((m) => m.id === around)
    if (index < 0) throw new Error('Message not found.')
    const start = Math.max(0, index - Math.floor((size - 1) / 2))
    const stop = Math.min(messages.length, start + size)
    return {
      items: messages.slice(start, stop),
      nextCursor: start > 0 ? messages[start]!.id : null,
      ...(stop < messages.length ? { hasNewer: true } : {}),
    }
  }
  let end = before ? messages.findIndex((m) => m.id === before) : messages.length
  if (end < 0) throw new Error('This history cursor has expired. Reload the conversation.')
  const start = Math.max(0, end - Math.min(100, Math.max(1, limit)))
  return { items: messages.slice(start, end), nextCursor: start > 0 ? messages[start]!.id : null }
}
export function appendMessage(session: DemoSession, message: Message) {
  if (!message.upstreamId && ['message', 'action'].includes(message.kind))
    message.upstreamId = `demo-${message.id}`
  const messages = (session.messages[message.conversationId] ??= [])
  messages.push(message)
  const cap = session.historyLimits?.[message.conversationId] ?? limits.messages
  if (messages.length > cap) messages.splice(0, messages.length - cap)
  // Unread and mention counts live on the server so they survive reloads. Clients count the
  // same events locally, so no conversation.updated is sent for these increments.
  const c = session.bootstrap.conversations.find((x) => x.id === message.conversationId)
  if (c && message.senderId !== 'you' && message.kind !== 'event') {
    c.unreadCount++
    const nick = session.bootstrap.networks.find((n) => n.id === c.networkId)?.nick
    if (c.kind === 'channel' && mentionsNick(stripFormatting(message.text), nick)) c.mentionCount++
  }
  emit(session, { type: 'message.created', data: message })
  recordInbox(session, message)
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
  const account = network.account
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
    upstreamId: `demo-${input.clientId}`,
    conversationId: conversation.id,
    senderId: 'you',
    ...(account ? { account } : {}),
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
    const echoed = session.messages[conversation.id]?.at(-1)
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
/** Demo counterpart of Go's attract: your newest message here sparkles; nothing is stored. */
export function attract(session: DemoSession, conversationId: string) {
  const c = getConversation(session, conversationId)
  if (c.kind !== 'channel') throw new Error('/attract works inside a channel')
  if (!/[~&@]/.test(c.memberModes?.you ?? ''))
    throw new ForbiddenError('/attract needs channel operator rights (op, admin or owner)')
  const message = (session.messages[conversationId] ?? []).findLast(
    (m) => m.senderId === 'you' && !m.deleted && ['message', 'action', 'notice'].includes(m.kind),
  )
  if (!message) throw new Error('send a message in this channel first, then /attract it')
  for (const listener of session.listeners)
    listener({
      v: 1,
      type: 'attract',
      data: { conversationId, memberId: 'you', messageId: message.id },
    })
  return message
}
/** Demo counterpart of Go's redact-message: only your own messages, files kept. */
export function redactMessage(session: DemoSession, conversationId: string, messageId: string) {
  getConversation(session, conversationId)
  const list = session.messages[conversationId] ?? []
  const message = list.find((m) => m.id === messageId)
  if (!message || message.kind === 'event') throw new Error('Message not found.')
  if (message.senderId !== 'you')
    throw new ForbiddenError('Only the author can delete this message.')
  if (message.deleted) throw new Error('This message was already deleted.')
  const attachment = message.attachment
  const deleted: Message = {
    ...message,
    text: 'Message deleted',
    deleted: true,
    reactions: [],
    gif: undefined,
    attachment: undefined,
    replyTo: undefined,
  }
  for (const [i, m] of list.entries()) {
    if (m.id === messageId) list[i] = deleted
    else if (m.replyTo?.id === messageId) {
      list[i] = { ...m, replyTo: { ...m.replyTo, text: deleted.text } }
      emit(session, { type: 'message.updated', data: list[i]! })
    }
  }
  emit(session, { type: 'message.updated', data: deleted })
  const upload = attachment
    ? [...session.uploads.values()].find((u) => u.url === attachment.url)
    : undefined
  return {
    message: deleted,
    attachmentRetained: !!attachment,
    ...(upload ? { uploadId: upload.id } : {}),
  }
}
/** Demo counterpart of Go's edit-message: only your own text messages, with history. */
export function editMessage(
  session: DemoSession,
  conversationId: string,
  messageId: string,
  text: string,
) {
  const conversation = getConversation(session, conversationId)
  const network = session.bootstrap.networks.find((n) => n.id === conversation.networkId)
  const list = session.messages[conversationId] ?? []
  const message = list.find((m) => m.id === messageId)
  if (!message || message.kind === 'event') throw new Error('Message not found.')
  if (message.senderId !== 'you') throw new ForbiddenError('Only the author can edit this message.')
  if (message.deleted) throw new Error('This message was already deleted.')
  if (!network?.features.edit) throw new Error('This network does not support editing messages.')
  if (message.kind !== 'message' && message.kind !== 'action')
    throw new Error('This message cannot be edited.')
  if (message.gif || message.attachment) throw new Error('GIFs and uploads cannot be edited.')
  if (!text.trim() || text.length > 4000) throw new Error('Invalid message.')
  if (text === message.text) throw new Error('The edited text is the same as the message.')
  const at = new Date().toISOString()
  const edits = [
    ...(message.edits ?? []),
    { upstreamId: `demo-edit-${crypto.randomUUID()}`, text, at },
  ].slice(-20)
  const edited: Message = {
    ...message,
    text,
    editedAt: at,
    originalText: message.originalText ?? message.text,
    edits,
  }
  for (const [i, m] of list.entries()) {
    if (m.id === messageId) list[i] = edited
    else if (m.replyTo?.id === messageId) {
      list[i] = { ...m, replyTo: { ...m.replyTo, text } }
      emit(session, { type: 'message.updated', data: list[i]! })
    }
  }
  emit(session, { type: 'message.updated', data: edited })
  return { message: edited }
}
export function deleteUpload(session: DemoSession, id: string) {
  if (!session.uploads.delete(id)) throw new Error('Upload not found.')
}
export class ForbiddenError extends Error {}
/** A personal write of a channel-wide value; the Go backend answers 422. */
export class ChannelSettingError extends Error {}

/** Like the Go backend: channel-wide settings, changed by channel operators only. */
export function updateChannelSettings(
  session: DemoSession,
  id: string,
  input: { notifications?: string },
) {
  const c = getConversation(session, id)
  if (c.kind !== 'channel') throw new Error('Conversation not found.')
  if (!/[~&@]/.test(c.memberModes?.you ?? ''))
    throw new ForbiddenError('Changing channel settings needs channel operator status.')
  const current = c.channelSettings ?? {
    notifications: 'default',
  }
  const notifications = input.notifications ?? current.notifications
  if (!['default', 'all', 'mentions', 'none'].includes(notifications))
    throw new Error('Notifications must be default, all, mentions or none.')
  c.channelSettings = {
    notifications: notifications as NonNullable<Conversation['channelSettings']>['notifications'],
    updatedBy: session.bootstrap.networks.find((n) => n.id === c.networkId)?.nick,
    updatedAt: new Date().toISOString(),
  }
  emit(session, { type: 'conversation.updated', data: wireConversation(c) })
  return wireConversation(c)
}
export function patchConversation(
  session: DemoSession,
  id: string,
  input: { read?: boolean; unreadFrom?: string; favorite?: boolean },
) {
  const c = getConversation(session, id)
  if (input.read) {
    markInboxRead(session, id)
    c.unreadCount = 0
    c.mentionCount = 0
    delete c.unreadFrom
  } else if (typeof input.unreadFrom === 'string') {
    // Like the Go backend: the chosen message, then what others sent after it. Only
    // conversation messages count; system and event rows don't.
    const counted = ['message', 'action', 'notice']
    const messages = (session.messages[id] ?? []).filter((m) => counted.includes(m.kind))
    const from = messages.findIndex((m) => m.id === input.unreadFrom)
    if (from < 0) throw new Error('That message is no longer available.')
    const unread = messages.slice(from).filter((m, i) => i === 0 || m.senderId !== 'you')
    const nick = session.bootstrap.networks.find((n) => n.id === c.networkId)?.nick
    c.unreadFrom = input.unreadFrom
    markInboxRead(session, id, input.unreadFrom)
    c.unreadCount = unread.length
    c.mentionCount =
      c.kind === 'channel'
        ? unread.filter((m) => m.senderId !== 'you' && mentionsNick(stripFormatting(m.text), nick))
            .length
        : 0
  }
  if (typeof input.favorite === 'boolean') c.favorite = input.favorite
  // Like the Go backend: reading and marking unread send only the counts.
  if (typeof input.favorite === 'boolean')
    emit(session, { type: 'conversation.updated', data: wireConversation(c) })
  else
    emit(session, {
      type: 'conversation.counts',
      data: {
        id: c.id,
        unreadCount: c.unreadCount,
        mentionCount: c.mentionCount,
        ...(c.unreadFrom ? { unreadFrom: c.unreadFrom } : {}),
      },
    })
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
    features: {
      setname: true,
      metadata: [],
      away: true,
      redact: true,
      edit: true,
      replies: true,
      reactions: true,
      typing: true,
      attract: true,
      readMarkers: true,
      history: true,
      messageLineBytes: 310,
      multiline: { maxBytes: 4096, maxLines: 100 },
    },
    ...settings,
  }
  network.account = network.nick
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
  for (const [messageId, ref] of session.saved)
    if (ref.networkId === id) session.saved.delete(messageId)
  dropNetworkInbox(session, id)
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
    joined: true,
    networkId: input.networkId,
    topic: member
      ? `Your conversation with ${member.displayName}.`
      : 'A fresh start. Say something good.',
    unreadCount: 0,
    mentionCount: 0,
    favorite: false,
    memberIds: member ? ['you', member.id, 'echo'] : ['you', 'echo'],
    ...(member
      ? {}
      : {
          memberModes: { you: '@' },
          modes: 'nt',
          channelSettings: {
            notifications: 'default' as const,
          },
        }),
  }
  session.bootstrap.conversations.push(c)
  session.messages[c.id] = []
  emit(session, { type: 'conversation.updated', data: wireConversation(c) })
  if (!member) membershipEvent(session, c, { type: 'join', actorId: 'you' })
  return c
}

// ---- Saved messages (demo counterpart of Go's irc/saved.go) ----------------------------------

const savable = (m: Message) =>
  !m.deleted && (m.kind === 'message' || m.kind === 'action' || m.kind === 'notice')

function resolveSaved(session: DemoSession, ref: SavedRef): SavedItem {
  const item: SavedItem = { ...ref, status: 'unavailable' }
  const conversation = session.bootstrap.conversations.find((c) => c.id === ref.conversationId)
  const message = session.messages[ref.conversationId]?.find((m) => m.id === ref.messageId)
  if (!conversation || conversation.networkId !== ref.networkId || !message) return item
  const wire = wireConversation(conversation)
  if (message.deleted)
    return {
      ...item,
      status: 'deleted',
      conversation: wire,
      message: {
        id: message.id,
        conversationId: message.conversationId,
        senderId: message.senderId,
        createdAt: message.createdAt,
        kind: message.kind,
        text: '',
        deleted: true,
        reactions: [],
      },
    }
  return { ...item, status: 'available', conversation: wire, message: structuredClone(message) }
}

const savedClock = new WeakMap<DemoSession, number>()

export function saveMessage(
  session: DemoSession,
  conversationId: string,
  messageId: string,
): SavedItem {
  const conversation = getConversation(session, conversationId)
  const message = session.messages[conversationId]?.find((m) => m.id === messageId)
  if (!message) throw new Error('Message not found.')
  if (!savable(message)) throw new Error('Only delivered chat messages can be saved.')
  const existing = session.saved.get(messageId)
  if (existing) return resolveSaved(session, existing)
  if (session.saved.size >= limits.saved)
    throw new Error('You have reached the saved-message limit; remove some saved messages first.')
  // Strictly increasing save times keep “newest first” the order things were saved in.
  const at = Math.max(Date.now(), (savedClock.get(session) ?? 0) + 1)
  savedClock.set(session, at)
  const ref: SavedRef = {
    messageId,
    conversationId,
    networkId: conversation.networkId,
    savedAt: new Date(at).toISOString(),
  }
  session.saved.set(messageId, ref)
  emit(session, { type: 'saved.added', data: ref })
  return resolveSaved(session, ref)
}

export function unsaveMessage(session: DemoSession, messageId: string) {
  const ref = session.saved.get(messageId)
  if (!ref) return
  session.saved.delete(messageId)
  emit(session, { type: 'saved.removed', data: { messageId, conversationId: ref.conversationId } })
}

/** Newest first; the cursor is the last listed message id. */
export function listSaved(session: DemoSession, cursor = '', q = '', limit = 30): SavedPage {
  const refs = [...session.saved.values()].sort(
    (a, b) => b.savedAt.localeCompare(a.savedAt) || b.messageId.localeCompare(a.messageId),
  )
  let start = 0
  if (cursor) {
    start = refs.findIndex((r) => r.messageId === cursor) + 1
    if (start === 0) throw new Error('invalid cursor')
  }
  const needle = q.trim().toLowerCase()
  const items: SavedItem[] = []
  let next: string | null = null
  for (const ref of refs.slice(start)) {
    const item = resolveSaved(session, ref)
    if (
      needle &&
      (item.status !== 'available' || !item.message?.text.toLowerCase().includes(needle))
    )
      continue
    if (items.length === Math.min(100, Math.max(1, limit))) {
      next = items.at(-1)!.messageId
      break
    }
    items.push(item)
  }
  return { items, nextCursor: next }
}

export const savedRefs = (session: DemoSession) => ({
  items: [...session.saved.values()].sort((a, b) => b.savedAt.localeCompare(a.savedAt)),
})
