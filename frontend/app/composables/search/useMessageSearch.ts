import { useQuery } from '@pinia/colada'
import type { MaybeRefOrGetter } from 'vue'
import type { SearchResult } from '../../../shared/types/chat'
import { canSearch, type SearchFilters } from '../../../shared/utils/search-filter'
import { messageSearchQuery } from '../../queries/chat'
import { readSearch, offlineStatus } from '../../utils/offline-archive'
import { useCursorPages } from '../shared/useCursorPages'
import { effectiveSearchSource, searchCacheKey, type SearchSourceMode } from './search-source'

/** Shared account-scoped search and paging for both message search surfaces. */
export function useMessageSearch(
  text: MaybeRefOrGetter<string>,
  filters: MaybeRefOrGetter<SearchFilters>,
  source: MaybeRefOrGetter<SearchSourceMode> = 'retained',
) {
  const effectiveSource = computed(() =>
    effectiveSearchSource(toValue(source), toValue(filters).in.length, offlineStatus.value.offline),
  )
  const key = computed(() =>
    JSON.stringify(
      searchCacheKey(
        toValue(text),
        toValue(filters),
        offlineStatus.value.offline,
        effectiveSource.value,
      ),
    ),
  )
  const enabled = computed(() => canSearch(toValue(text), toValue(filters)))
  const search = useQuery(() => ({
    ...messageSearchQuery({
      text: toValue(text),
      filters: toValue(filters),
      source: effectiveSource.value,
    }),
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
    load: (cursor, signal) =>
      readSearch(toValue(text), toValue(filters), cursor, signal, effectiveSource.value),
    enabled,
    duplicates: 'keep-last',
  })
  return { search, results, cursor, loadMore, loadingMore, moreError, source: effectiveSource }
}
