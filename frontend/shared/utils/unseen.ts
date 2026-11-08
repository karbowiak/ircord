/** The newest message the reader had when they scrolled up. */
export type SeenUpTo = { id: string; createdAt: string }

/**
 * How many messages arrived after the reader left the latest message: the counted items after
 * `seenUpTo`. Older history loaded above the view while reading back is not news. `items` is the
 * conversation's history in order, still holding deleted messages, so the anchor stays findable
 * when it is deleted. If the anchor has left the window (trimmed while reading far back), items
 * newer than it count instead.
 */
export function unseenSince<T extends { id: string; createdAt: string }>(
  items: readonly T[],
  seenUpTo: SeenUpTo | undefined,
  counts: (item: T) => boolean,
): number {
  if (!seenUpTo) return 0
  const index = items.findLastIndex((m) => m.id === seenUpTo.id)
  const after =
    index >= 0
      ? items.slice(index + 1)
      : items.filter((m) => Date.parse(m.createdAt) > Date.parse(seenUpTo.createdAt))
  return after.filter(counts).length
}
