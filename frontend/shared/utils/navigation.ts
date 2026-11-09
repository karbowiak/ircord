import type { Conversation } from '../types/chat'

/**
 * The next conversation needing attention from `activeId` in `direction`: conversations with
 * mentions when there are any, otherwise any unread one. Null when nothing is unread.
 */
export function nextUnread(
  conversations: Pick<Conversation, 'id' | 'mentionCount' | 'unreadCount'>[],
  activeId: string,
  direction: 1 | -1,
) {
  const waiting = conversations.filter((c) => c.mentionCount || c.unreadCount)
  if (!waiting.length) return null
  const pool = waiting.some((c) => c.mentionCount) ? waiting.filter((c) => c.mentionCount) : waiting
  const index = pool.findIndex((c) => c.id === activeId)
  // Not in the pool: ↓ starts at the first entry, ↑ at the last.
  if (index < 0) return direction === 1 ? pool[0]! : pool.at(-1)!
  return pool[(index + direction + pool.length) % pool.length]!
}

/** The network after `activeId` in `direction`, in rail order, wrapping around; null without one. */
export function nextNetwork<T extends { id: string }>(
  networks: readonly T[],
  activeId: string,
  direction: 1 | -1,
): T | null {
  if (!networks.length) return null
  const index = networks.findIndex((n) => n.id === activeId)
  if (index < 0) return direction === 1 ? networks[0]! : networks.at(-1)!
  const next = networks[(index + direction + networks.length) % networks.length]!
  return next.id === activeId ? null : next
}
