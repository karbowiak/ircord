import { useQuery } from '@pinia/colada'
import type { MaybeRefOrGetter } from 'vue'
import type { SearchResult } from '../../../shared/types/chat'
import { canSearch, searchParams, type SearchFilters } from '../../../shared/utils/search-filter'
import { messageSearchQuery } from '../../queries/chat'
import { readSearch, offlineStatus } from '../../utils/offline-archive'
import { useCursorPages } from '../shared/useCursorPages'

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
  const {
    items: results,
    cursor,
    loadMore,
    loadingMore,
    moreError,
  } = useCursorPages<SearchResult>({
    firstPage: search.data,
    resetOn: [key, search.data],
    itemKey: (result) => result.message.id,
    load: (cursor, signal) => readSearch(toValue(text), toValue(filters), cursor, signal),
    enabled,
    duplicates: 'keep-last',
  })
  return { search, results, cursor, loadMore, loadingMore, moreError }
}
