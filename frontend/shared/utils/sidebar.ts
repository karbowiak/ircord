/**
 * Collapsible sections of the conversation list. Each network keeps its own collapsed
 * sections, like categories in Discord, so a section is named by network and section id.
 */

/** Most collapsed sections remembered on a device; the oldest are forgotten first. */
export const maxCollapsedSections = 200

export const sectionKey = (networkId: string, sectionId: string) => `${networkId}/${sectionId}`

/** Cleans a stored list: strings only, no duplicates, bounded in count and length. */
export function sanitizeCollapsed(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  const keys = value.filter(
    (key): key is string => typeof key === 'string' && key.length > 0 && key.length <= 300,
  )
  return [...new Set(keys)].slice(-maxCollapsedSections)
}

/** The list with `key` collapsed (added) or expanded (removed). */
export function toggleCollapsed(list: readonly string[], key: string): string[] {
  return list.includes(key) ? list.filter((k) => k !== key) : sanitizeCollapsed([...list, key])
}

/**
 * What a collapsed section still lists: the open conversation, so you don't lose your place,
 * and conversations that want attention (mentions, or unread ones that aren't muted).
 */
export function collapsedItems<T extends { id: string; unreadCount: number; mentionCount: number }>(
  items: readonly T[],
  activeId: string,
  muted: (item: T) => boolean,
): T[] {
  return items.filter(
    (c) => c.id === activeId || c.mentionCount > 0 || (c.unreadCount > 0 && !muted(c)),
  )
}
