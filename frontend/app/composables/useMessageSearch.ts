import { useQuery } from '@pinia/colada'
import type { MaybeRefOrGetter } from 'vue'
import type { SearchResult } from '../../shared/types/chat'
import { canSearch, searchParams, type SearchFilters } from '../../shared/utils/search-filter'
import { messageSearchQuery } from '../queries/chat'
import { readSearch, offlineStatus } from '../utils/offline-archive'

/** Shared account-scoped search and paging for both message search surfaces. */
export function useMessageSearch(
  text: MaybeRefOrGetter<string>,
  filters: MaybeRefOrGetter<SearchFilters>,
) {
  const params = computed(() => searchParams(toValue(text), toValue(filters)))
  const key = computed(() => JSON.stringify([params.value, offlineStatus.value.offline]))
  const enabled = computed(() => canSearch(toValue(text), toValue(filters)))
  const search = useQuery(() => ({
    ...messageSearchQuery({ text: toValue(text), filters: toValue(filters) }),
    enabled: enabled.value,
  }))
  const older = ref<SearchResult[]>([])
  const olderCursor = ref<string | null>()
  const loadingMore = ref(false)
  const moreError = ref<unknown>('')
  let generation = 0
  watch(
    [key, search.data],
    () => {
      generation++
      older.value = []
      olderCursor.value = undefined
      moreError.value = ''
    },
    { flush: 'sync' },
  )
  const cursor = computed(() =>
    enabled.value
      ? olderCursor.value === undefined
        ? search.data.value?.nextCursor
        : olderCursor.value
      : null,
  )
  const results = computed(() =>
    enabled.value
      ? [
          ...new Map(
            [...(search.data.value?.items ?? []), ...older.value].map((r) => [r.message.id, r]),
          ).values(),
        ]
      : [],
  )
  async function loadMore() {
    if (!cursor.value || loadingMore.value) return
    const revision = generation
    loadingMore.value = true
    moreError.value = ''
    try {
      const page = await readSearch(toValue(text), toValue(filters), cursor.value)
      if (revision !== generation) return
      older.value = [...older.value, ...page.items]
      olderCursor.value = page.nextCursor
    } catch (error) {
      if (revision === generation) moreError.value = error
    } finally {
      loadingMore.value = false
    }
  }
  return { search, results, cursor, loadMore, loadingMore, moreError }
}
