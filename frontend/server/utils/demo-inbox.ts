// Demo counterpart of Go's irc/inbox.go: the private attention inbox of one demo session.
// It models mentions of the network nick, replies to your messages, direct messages and a
// channel level of "none". Personal notification settings (highlight words, ignores, a
// direct message's own level) live in async account storage and are not applied here.
import type { InboxFilter, InboxItem, InboxPage, InboxReason, Message } from '../../shared/types/chat'
import { mentionsNick } from '../../shared/utils/identity'
import { stripFormatting } from '../../shared/utils/irc'
import { groupMentioned, recipientIn } from '../../shared/group-mentions'
import { emit, type DemoSession } from './demo-store'

export const INBOX_LIMIT = 500
const EXCERPT = 280

export interface DemoInboxEntry {
  messageId: string
  conversationId: string
  networkId: string
  senderId: string
  reasons: InboxReason[]
  createdAt: string
  dismissed: boolean
}

const counted = (m: Message) => m.kind === 'message' || m.kind === 'action' || m.kind === 'notice'

/** Why a message needs your attention, like Go's notify.Policy.Attention. */
export function attention(session: DemoSession, message: Message): InboxReason[] {
  const c = session.bootstrap.conversations.find((x) => x.id === message.conversationId)
  if (!c || message.senderId === 'you' || !counted(message) || message.deleted || message.historical)
    return []
  if (c.kind === 'channel' && c.channelSettings?.notifications === 'none') return []
  const network = session.bootstrap.networks.find((n) => n.id === c.networkId)
  const you = recipientIn(c, network, session.bootstrap.user, message.senderId)
  const reasons: InboxReason[] = []
  const text = stripFormatting(message.text)
  if (mentionsNick(text, network?.nick) || groupMentioned(text, you)) reasons.push('mention')
  if (message.replyTo) {
    const parent = session.messages[c.id]?.find((m) => m.id === message.replyTo!.id)
    if (parent?.senderId === 'you' && !parent.deleted) reasons.push('reply')
  }
  if (c.kind === 'direct') reasons.push('direct')
  return reasons
}

/** Adds a qualifying message once; replays and repeats change nothing. */
export function recordInbox(session: DemoSession, message: Message) {
  const reasons = attention(session, message)
  if (!reasons.length || session.inbox.has(message.id)) return
  const c = session.bootstrap.conversations.find((x) => x.id === message.conversationId)!
  const entry: DemoInboxEntry = {
    messageId: message.id,
    conversationId: c.id,
    networkId: c.networkId,
    senderId: message.senderId,
    reasons,
    createdAt: message.createdAt,
    dismissed: false,
  }
  session.inbox.set(message.id, entry)
  // Oldest beyond the limit go first.
  const excess = session.inbox.size - INBOX_LIMIT
  if (excess > 0)
    for (const e of [...session.inbox.values()]
      .sort((a, b) => a.createdAt.localeCompare(b.createdAt))
      .slice(0, excess))
      session.inbox.delete(e.messageId)
  const { dismissed: _, ...data } = entry
  emit(session, { type: 'inbox.added', data })
}

/** The fixture conversations' read positions follow their seeded unread counts. */
export function seedInbox(session: DemoSession) {
  for (const c of session.bootstrap.conversations) {
    const list = (session.messages[c.id] ?? []).filter(counted)
    const read = list[list.length - 1 - c.unreadCount]
    if (read) session.readAt[c.id] = read.createdAt
    for (const m of list) {
      const reasons = attention(session, m)
      if (reasons.length)
        session.inbox.set(m.id, {
          messageId: m.id,
          conversationId: c.id,
          networkId: c.networkId,
          senderId: m.senderId,
          reasons,
          createdAt: m.createdAt,
          dismissed: false,
        })
    }
  }
}

/** Reading a conversation moves its read position; a manual unread boundary moves it back. */
export function markInboxRead(session: DemoSession, conversationId: string, unreadFrom?: string) {
  const list = (session.messages[conversationId] ?? []).filter(counted)
  if (unreadFrom) {
    const from = list.find((m) => m.id === unreadFrom)
    if (from) session.readAt[conversationId] = new Date(Date.parse(from.createdAt) - 1).toISOString()
  } else if (list.length) session.readAt[conversationId] = list[list.length - 1]!.createdAt
}

function excerpt(text: string) {
  const plain = [...stripFormatting(text).split(/\s+/).filter(Boolean).join(' ')]
  return plain.length <= EXCERPT
    ? { excerpt: plain.join('') }
    : { excerpt: `${plain.slice(0, EXCERPT).join('').trim()}…`, truncated: true }
}

function resolve(session: DemoSession, e: DemoInboxEntry): InboxItem {
  const base = {
    messageId: e.messageId,
    conversationId: e.conversationId,
    networkId: e.networkId,
    reasons: e.reasons,
    createdAt: e.createdAt,
    dismissed: e.dismissed,
    unread: false,
  }
  const c = session.bootstrap.conversations.find((x) => x.id === e.conversationId)
  const m = session.messages[e.conversationId]?.find((x) => x.id === e.messageId)
  if (!c || c.networkId !== e.networkId || !m) return { ...base, status: 'unavailable' }
  const kind = m.kind as 'message' | 'action' | 'notice'
  if (m.deleted) return { ...base, senderId: e.senderId, kind, status: 'deleted' }
  const read = session.readAt[e.conversationId]
  return {
    ...base,
    senderId: e.senderId,
    kind,
    status: 'available',
    unread: !read || e.createdAt > read,
    ...excerpt(m.text),
  }
}

const matches = (filter: InboxFilter, item: InboxItem) => {
  if (filter === 'dismissed') return item.dismissed
  if (item.dismissed) return false
  if (filter === 'unread') return item.unread
  if (filter === 'mentions') return item.reasons.includes('mention')
  if (filter === 'replies') return item.reasons.includes('reply')
  if (filter === 'direct') return item.reasons.includes('direct')
  return true
}

export const inboxUnread = (session: DemoSession) =>
  [...session.inbox.values()].filter((e) => matches('unread', resolve(session, e))).length

/** Newest first; the cursor is the last listed message id. */
export function listInbox(session: DemoSession, filter: InboxFilter = 'all', cursor = '', limit = 30): InboxPage {
  const all = [...session.inbox.values()]
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt) || b.messageId.localeCompare(a.messageId))
    .map((e) => resolve(session, e))
    .filter((item) => matches(filter, item))
  let start = 0
  if (cursor) {
    start = all.findIndex((i) => i.messageId === cursor) + 1
    if (start === 0) throw new Error('invalid cursor')
  }
  const size = Math.min(100, Math.max(1, limit))
  const items = all.slice(start, start + size)
  return {
    items,
    nextCursor: start + size < all.length ? items.at(-1)!.messageId : null,
    unread: inboxUnread(session),
  }
}

export function setInboxDismissed(session: DemoSession, messageId: string, dismissed: boolean) {
  const entry = session.inbox.get(messageId)
  if (!entry) throw new Error('Inbox item not found.')
  if (entry.dismissed === dismissed) return
  entry.dismissed = dismissed
  emit(session, { type: 'inbox.updated', data: { messageId, dismissed } })
}

export function dropNetworkInbox(session: DemoSession, networkId: string) {
  for (const [id, e] of session.inbox) if (e.networkId === networkId) session.inbox.delete(id)
}
