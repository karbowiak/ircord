// /attract: which timeline row sparkles, and for how long.

/**
 * The whole effect: a sparkle on the avatar and a shine across the message. The CSS animations
 * take their length from `--attract-ms` (see `attractClock`), and the Go backend's receive
 * coalescing window (`attractEffect` in backend/internal/irc/attract.go) matches it.
 */
export const ATTRACT_MS = 3000

/**
 * CSS variables that place an element's animation on the effect's clock. Compute this once when
 * the element starts showing the effect: a running animation measures a changed delay from its
 * original start, so recomputing it on a re-render would make the animation jump ahead.
 */
export function attractClock(startedAt: number, now = Date.now()) {
  return { '--attract-delay': `${startedAt - now}ms`, '--attract-ms': `${ATTRACT_MS}ms` }
}

export interface AttractTarget {
  messageId: string
  memberId: string
}

/** The slice of a timeline row this needs; `undefined` stands for a non-message row. */
export interface AttractRow {
  id: string
  senderId: string
}

/**
 * The row that shines and the row whose avatar sparkles. A grouped continuation has no avatar,
 * so the sparkle goes to the head of its group. When the exact message isn't loaded, the
 * sender's newest loaded message stands in; with none, nothing plays.
 */
export function resolveAttract(
  rows: readonly (AttractRow | undefined)[],
  target: AttractTarget,
  grouped: (index: number) => boolean,
): { row: number; avatar: number } | undefined {
  let row = rows.findLastIndex((r) => r?.id === target.messageId)
  if (row < 0) row = rows.findLastIndex((r) => r?.senderId === target.memberId)
  if (row < 0) return undefined
  let avatar = row
  while (avatar > 0 && grouped(avatar)) avatar--
  return { row, avatar }
}

/**
 * Where the target row is relative to the view: `undefined` when visible, else `above` or `below`.
 * `item` is its virtual item when rendered; otherwise its index is compared with the first one.
 */
export function offscreen(
  row: number,
  item: { start: number; size: number } | undefined,
  firstRendered: number,
  view: { scrollTop: number; clientHeight: number },
): 'above' | 'below' | undefined {
  if (item && rowVisible(item.start, item.size, view)) return undefined
  const start = item?.start ?? (row < firstRendered ? -Infinity : Infinity)
  return start < view.scrollTop ? 'above' : 'below'
}

/** Whether a row's box overlaps the visible part of the scroller. */
export function rowVisible(
  start: number,
  size: number,
  view: { scrollTop: number; clientHeight: number },
) {
  return start + size > view.scrollTop && start < view.scrollTop + view.clientHeight
}
