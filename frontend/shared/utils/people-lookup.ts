export interface PeopleScope {
  accountId: string
  networkId: string
  purpose: 'mention' | 'command' | 'mask' | 'history' | 'dm' | 'services' | 'party'
  conversationId?: string
  query: string
  limit: number
}
export interface PeopleLookupPage<T> {
  items: T[]
  source: 'known' | 'registered'
  status: 'ready' | 'partial' | 'unavailable'
  more: boolean
}
export interface PeopleLookupState<T> {
  page?: PeopleLookupPage<T>
  loading: boolean
  error?: unknown
}

/** Cache only successful complete pages, never partial or failed lookups. */
export function createPeopleCache<T>(maxEntries = 100, ttl = 10_000) {
  let account = ''
  const entries = new Map<string, { page: PeopleLookupPage<T>; expires: number }>()
  const accountScope = (id: string) => {
    if (id !== account) {
      entries.clear()
      account = id
    }
  }
  const key = (scope: PeopleScope) =>
    JSON.stringify([
      scope.accountId,
      scope.networkId,
      scope.purpose,
      scope.conversationId ?? '',
      scope.query,
      scope.limit,
    ])
  return {
    clear() {
      account = ''
      entries.clear()
    },
    get(scope: PeopleScope) {
      accountScope(scope.accountId)
      const k = key(scope)
      const entry = entries.get(k)
      if (!entry || entry.expires <= Date.now()) {
        entries.delete(k)
        return undefined
      }
      entries.delete(k)
      entries.set(k, entry)
      return entry.page
    },
    put(scope: PeopleScope, page: PeopleLookupPage<T>) {
      accountScope(scope.accountId)
      if (page.status !== 'ready') return
      const k = key(scope)
      entries.delete(k)
      entries.set(k, { page, expires: Date.now() + ttl })
      while (entries.size > maxEntries) entries.delete(entries.keys().next().value!)
    },
  }
}

/** One owner for debounce, abort and stale-response fencing, including transports that ignore abort. */
export function createPeopleSearch<T>(
  cache: ReturnType<typeof createPeopleCache<T>>,
  load: (scope: PeopleScope, signal: AbortSignal) => Promise<PeopleLookupPage<T>>,
  update: (state: PeopleLookupState<T>) => void,
  delay = 200,
) {
  let revision = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: AbortController | undefined
  function cancel() {
    revision++
    clearTimeout(timer)
    controller?.abort()
    controller = undefined
  }
  function search(scope: PeopleScope | null) {
    cancel()
    if (!scope?.accountId || !scope.networkId) {
      update({ loading: false })
      return
    }
    const cached = cache.get(scope)
    if (cached) {
      update({ page: cached, loading: false })
      return
    }
    const current = revision
    update({ loading: true })
    timer = setTimeout(async () => {
      const request = new AbortController()
      controller = request
      try {
        const page = await load(scope, request.signal)
        if (current !== revision || request.signal.aborted) return
        cache.put(scope, page)
        update({ page, loading: false })
      } catch (error) {
        if (current !== revision || request.signal.aborted) return
        update({ loading: false, error })
      }
    }, delay)
  }
  return { search, cancel }
}
