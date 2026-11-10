import { computed, onScopeDispose, ref, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter, WatchSource } from 'vue'

export interface CursorPage<T> {
  items: readonly T[]
  nextCursor?: string | null
}

export interface CursorPagesOptions<T> {
  firstPage: MaybeRefOrGetter<CursorPage<T> | undefined>
  /** Domain changes that invalidate older pages; first-page edits need not reset them. */
  resetOn: WatchSource[]
  itemKey: (item: T) => string
  load: (cursor: string, signal: AbortSignal) => Promise<CursorPage<T>>
  enabled?: MaybeRefOrGetter<boolean>
  /** Search keeps later versions; collections keep the first listed item. */
  duplicates?: 'keep-first' | 'keep-last'
}

/** Older-page state shared by collections and search, without owning their queries or filters. */
export function useCursorPages<T>(options: CursorPagesOptions<T>) {
  const older = shallowRef<T[]>([])
  // Undefined means no older page was read. Null records a terminal page, even when empty.
  const olderCursor = ref<string | null>()
  const loadingMore = ref(false)
  const moreError = shallowRef<unknown>('')
  const enabled = () => toValue(options.enabled ?? true)
  const keepLast = options.duplicates === 'keep-last'
  let generation = 0
  let flight: AbortController | undefined
  let disposed = false

  const cursor = computed(() =>
    enabled()
      ? olderCursor.value === undefined
        ? (toValue(options.firstPage)?.nextCursor ?? null)
        : olderCursor.value
      : null,
  )
  const items = computed(() => {
    if (!enabled()) return []
    const unique = new Map<string, T>()
    for (const item of [...(toValue(options.firstPage)?.items ?? []), ...older.value]) {
      const key = options.itemKey(item)
      if (keepLast || !unique.has(key)) unique.set(key, item)
    }
    return [...unique.values()]
  })

  function reset() {
    generation++
    flight?.abort()
    flight = undefined
    older.value = []
    olderCursor.value = undefined
    loadingMore.value = false
    moreError.value = ''
  }
  watch([...options.resetOn, enabled], reset, { flush: 'sync' })
  onScopeDispose(() => {
    disposed = true
    reset()
  })

  async function loadMore() {
    const nextCursor = cursor.value
    if (disposed || !nextCursor || loadingMore.value) return
    const revision = generation
    const controller = new AbortController()
    flight = controller
    loadingMore.value = true
    moreError.value = ''
    try {
      const page = await options.load(nextCursor, controller.signal)
      if (revision !== generation) return
      const known = new Set(items.value.map(options.itemKey))
      const next = new Map(older.value.map((item) => [options.itemKey(item), item]))
      for (const item of page.items) {
        const key = options.itemKey(item)
        if (keepLast || !known.has(key)) {
          next.set(key, item)
          known.add(key)
        }
      }
      older.value = [...next.values()]
      olderCursor.value = page.nextCursor ?? null
    } catch (error) {
      if (revision === generation) moreError.value = error
    } finally {
      // An obsolete request cannot release a replacement request's loading state.
      if (flight === controller) {
        flight = undefined
        loadingMore.value = false
      }
    }
  }

  return { older, items, cursor, loadMore, loadingMore, moreError, reset }
}
