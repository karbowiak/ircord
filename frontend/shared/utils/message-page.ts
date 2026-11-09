import type { Message, MessagePage } from '../types/chat'

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
    if ((message.liveSeq ?? 0) > (incoming.cursor ?? 0) && (!detached || items.has(message.id)))
      items.set(message.id, message)
  }
  return {
    ...incoming,
    items: [...items.values()].sort(compareMessages).slice(-500),
    detached,
  }
}

export function compareMessages(a: Message, b: Message) {
  return (
    Date.parse(a.createdAt) - Date.parse(b.createdAt) ||
    (a.position ?? 0) - (b.position ?? 0) ||
    (a.id < b.id ? -1 : a.id > b.id ? 1 : 0)
  )
}
