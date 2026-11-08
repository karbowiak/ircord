/**
 * How many messages arrived after the reader left the latest message: those after `seenUpTo`,
 * the newest message they had when they scrolled up. Older history loaded above the view while
 * reading back is not news, so only messages after that one count. Without that message in the
 * list (none yet, or it left the window) nothing is counted.
 */
export function unseenSince(ids: readonly string[], seenUpTo: string | undefined): number {
  if (!seenUpTo) return 0
  const index = ids.lastIndexOf(seenUpTo)
  return index < 0 ? 0 : ids.length - 1 - index
}
