import type { components } from '../../../shared/generated/api'
import type { SearchFilters } from '../../../shared/utils/search-filter'
import { searchParams } from '../../../shared/utils/search-filter'

export type SearchSourceMode = 'retained' | 'upstream'
export type SearchSourceInfo = components['schemas']['SearchSource']
type SearchPageSource = { source?: SearchSourceInfo; offline?: boolean }

const fallbackReasonKeys: Record<NonNullable<SearchSourceInfo['reason']>, string> = {
  'unsupported-scope': 'collectionsUi.search.fallbackUnsupportedScope',
  'unsupported-filters': 'collectionsUi.search.fallbackUnsupportedFilters',
  'capability-unavailable': 'collectionsUi.search.fallbackCapabilityUnavailable',
  'upstream-unavailable': 'collectionsUi.search.fallbackUpstreamUnavailable',
  continuation: 'collectionsUi.search.fallbackContinuation',
}

export function effectiveSearchSource(
  requested: SearchSourceMode,
  conversationCount: number,
  offline: boolean,
): SearchSourceMode {
  return requested === 'upstream' && conversationCount === 1 && !offline ? 'upstream' : 'retained'
}

export function searchCacheKey(
  text: string,
  filters: SearchFilters,
  offline: boolean,
  source: SearchSourceMode,
) {
  return [
    'search',
    'messages',
    offline ? 'offline' : 'online',
    source,
    text,
    JSON.stringify(searchParams('', filters)),
  ]
}

export function searchRequestParams(
  text: string,
  filters: SearchFilters,
  cursor: string | undefined,
  source: SearchSourceMode,
) {
  return { ...searchParams(text, filters), cursor, source }
}

export function searchCoverageKey(page?: SearchPageSource): string | undefined {
  if (page?.offline || !page?.source) return undefined
  const { actual, coverage, completeness, reason } = page.source
  if (completeness !== 'unknown') return undefined
  if (actual === 'upstream' && coverage === 'upstream-bounded')
    return 'collectionsUi.search.coverageUpstream'
  if (actual !== 'retained' || coverage !== 'retained-indexed') return undefined
  if (!reason || !Object.hasOwn(fallbackReasonKeys, reason))
    return 'collectionsUi.search.coverageRetained'
  return fallbackReasonKeys[reason]
}

export function isSearchResultJumpable(result: {
  context?: { available?: boolean; identity?: string }
}): boolean {
  return result.context?.available !== false
}

export function searchResultAuthor(
  result: { context?: { available?: boolean; identity?: string }; senderNick?: string },
  member?: { displayName: string },
): string | undefined {
  if (member) return member.displayName
  return result.context?.identity === 'ephemeral' ? result.senderNick : undefined
}

export interface SearchPageFallback<T> {
  offline: boolean
  cursor?: string
  signal?: AbortSignal
  archiveAvailable: boolean
  archive: () => Promise<T>
  server: () => Promise<T>
  isUnavailable: (error: unknown) => boolean
}

/** Keeps cancellation and offline continuations out of network fallback handling. */
export async function loadSearchPage<T>(options: SearchPageFallback<T>): Promise<T> {
  const { offline, cursor, signal, archiveAvailable, archive, server, isUnavailable } = options
  if (signal?.aborted)
    throw signal.reason ?? Object.assign(new Error('Search was canceled.'), { name: 'AbortError' })
  if (offline || cursor?.startsWith('offline-search:')) return archive()
  try {
    return await server()
  } catch (error) {
    if (signal?.aborted || !isUnavailable(error) || !archiveAvailable) throw error
    return archive()
  }
}
