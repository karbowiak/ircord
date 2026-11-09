/**
 * Your own order for conversation-list sections. A section without a stored order lists its
 * conversations A to Z. After you drag or move one, the section keeps the order you made, and
 * conversations it does not know yet (new joins, new favorites) go to the bottom.
 * Orders are keyed by sectionKey(network, section) and follow the account in preferences.
 */

/** Sections you can reorder. Direct messages keep their own order. */
export const sortableSections = ['favorites', 'channels'] as const
export type SortableSection = (typeof sortableSections)[number]
export const isSortableSection = (id: string): id is SortableSection =>
  (sortableSections as readonly string[]).includes(id)

export type SidebarOrder = Record<string, string[]>

/** Most stored section orders and conversations per order. */
export const maxSidebarOrders = 50
export const maxSidebarOrderIds = 200

const validString = (value: unknown, max: number): value is string =>
  typeof value === 'string' && value.length > 0 && value.length <= max

/** Cleans a stored value: string lists only, no duplicates, bounded in count and length. */
export function sanitizeSidebarOrder(value: unknown): SidebarOrder {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
  const entries = Object.entries(value)
    .filter(([key, ids]) => validString(key, 300) && Array.isArray(ids))
    .map(([key, ids]) => [
      key,
      [...new Set((ids as unknown[]).filter((id): id is string => validString(id, 300)))].slice(
        0,
        maxSidebarOrderIds,
      ),
    ])
  return Object.fromEntries(entries.slice(-maxSidebarOrders))
}

/**
 * `items` in display order: A to Z without a stored order; otherwise the stored order, then
 * conversations the order does not list yet, A to Z, at the bottom.
 */
export function orderItems<T extends { id: string }>(
  items: readonly T[],
  order: readonly string[] | undefined,
  byName: (a: T, b: T) => number,
): T[] {
  const sorted = [...items].sort(byName)
  if (!order) return sorted
  const position = new Map(order.map((id, index) => [id, index]))
  const known = sorted.filter((c) => position.has(c.id))
  known.sort((a, b) => position.get(a.id)! - position.get(b.id)!)
  return [...known, ...sorted.filter((c) => !position.has(c.id))]
}

/**
 * The ids with `id` moved next to `target`: before it or after it. Returns the input unchanged
 * when either is missing or both are the same.
 */
export function moveId(
  ids: readonly string[],
  id: string,
  target: string,
  place: 'before' | 'after',
): string[] {
  if (id === target || !ids.includes(id) || !ids.includes(target)) return [...ids]
  const rest = ids.filter((other) => other !== id)
  const index = rest.indexOf(target) + (place === 'after' ? 1 : 0)
  return [...rest.slice(0, index), id, ...rest.slice(index)]
}

/** The ids with `id` one step up (-1) or down (1); unchanged at either end. */
export function stepId(ids: readonly string[], id: string, step: -1 | 1): string[] {
  const index = ids.indexOf(id)
  const target = ids[index + step]
  if (index < 0 || !target) return [...ids]
  return moveId(ids, id, target, step < 0 ? 'before' : 'after')
}
