import type { Message, MessagePage } from '../types/chat'
import { messageBoundary } from './history-cursor'

export const MAX_TIMELINE_MESSAGES = 500

/** Keep pagination tied to the messages that survived a bounded newest-window trim. */
export function newestMessagePage(page: MessagePage, limit = MAX_TIMELINE_MESSAGES): MessagePage {
  if (page.items.length <= limit) return page
  const items = page.items.slice(-limit)
  return { ...page, items, nextCursor: messageBoundary(items[0]!.id) }
}

export function updateMessagePage(page: MessagePage, message: Message, seq = 0): MessagePage {
  const existing = page.items.find((item) => item.id === message.id)
  if ((page.detached || page.hasNewer) && !existing) {
    if (!seq || seq <= (page.newerLiveSeq ?? 0)) return page
    return { ...page, hasNewer: true, newerLiveSeq: seq }
  }
  if (seq && (existing?.liveSeq ?? 0) > seq) return page
  return newestMessagePage({
    ...page,
    items: [
      ...page.items.filter((item) => item.id !== message.id),
      { ...message, liveSeq: seq || existing?.liveSeq || 0 },
    ].sort(compareMessages),
  })
}

/** An older window that drops the live end must stop accepting unrelated new messages. */
export function prependMessagePage(
  current: MessagePage,
  older: MessagePage,
  limit = MAX_TIMELINE_MESSAGES,
): MessagePage {
  const items = new Map<string, Message>(
    older.items.map((message) => [message.id, { ...message, liveSeq: older.cursor ?? 0 }]),
  )
  for (const message of current.items) {
    const previous = items.get(message.id)
    if (!previous || (message.liveSeq ?? 0) > (previous.liveSeq ?? 0))
      items.set(message.id, message)
  }
  const ordered = [...items.values()].sort(compareMessages)
  const detached = !!current.detached || !!current.hasNewer || ordered.length > limit
  return {
    ...current,
    items: ordered.slice(0, limit),
    nextCursor: older.nextCursor,
    syncError: older.syncError,
    offline: !!older.offline,
    hasNewer: detached,
    detached,
  }
}

/** Advance through an overlapping around-window, trimming the older edge instead of the tail. */
export function appendMessagePage(
  current: MessagePage,
  newer: MessagePage,
  limit = MAX_TIMELINE_MESSAGES,
): MessagePage {
  const boundary = current.items.at(-1)
  if (!boundary || !newer.items.some((message) => message.id === boundary.id))
    throw new Error('History page did not include its boundary')
  if (newer.hasNewer && !newer.items.some((message) => compareMessages(message, boundary) > 0))
    throw new Error('History page did not advance')
  const items = new Map(current.items.map((message) => [message.id, message]))
  for (const message of newer.items) {
    const previous = items.get(message.id)
    if (
      (previous || compareMessages(message, boundary) > 0) &&
      (!previous || (previous.liveSeq ?? 0) <= (newer.cursor ?? 0))
    )
      items.set(message.id, { ...message, liveSeq: newer.cursor ?? 0 })
  }
  const pending = (current.newerLiveSeq ?? 0) > (newer.cursor ?? 0)
  const hasNewer =
    !!newer.hasNewer || (!newer.offline && pending) || !!newer.partial || !!newer.syncError
  return newestMessagePage(
    {
      ...current,
      items: [...items.values()].sort(compareMessages),
      cursor: newer.cursor,
      hasNewer,
      detached: hasNewer || !!newer.offline,
      partial: !!newer.partial,
      offline: !!newer.offline,
      syncError: newer.syncError,
      newerLiveSeq: newer.offline || pending ? current.newerLiveSeq : undefined,
    },
    limit,
  )
}

/** Keep socket revisions that committed after the history response's read cut. */
export function mergeMessagePage(
  incoming: MessagePage,
  current?: MessagePage,
  detached = false,
): MessagePage {
  const items = new Map<string, Message>(
    incoming.items.map((message) => [message.id, { ...message, liveSeq: incoming.cursor ?? 0 }]),
  )
  for (const message of current?.items ?? []) {
    if (
      (message.liveSeq ?? 0) > (incoming.cursor ?? 0) &&
      (!detached || items.has(message.id)) &&
      (items.has(message.id) ||
        !incoming.items.length ||
        compareMessages(message, incoming.items[0]!) >= 0)
    )
      items.set(message.id, message)
  }
  const pending = (current?.newerLiveSeq ?? 0) > (incoming.cursor ?? 0)
  return newestMessagePage({
    ...incoming,
    items: [...items.values()].sort(compareMessages),
    hasNewer: !!incoming.hasNewer || (!incoming.offline && pending),
    detached: detached || !!incoming.hasNewer || pending,
    newerLiveSeq: incoming.offline || pending ? current?.newerLiveSeq : undefined,
  })
}

export function compareMessages(a: Message, b: Message) {
  return (
    Date.parse(a.createdAt) - Date.parse(b.createdAt) ||
    (a.position ?? 0) - (b.position ?? 0) ||
    (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
  )
}
