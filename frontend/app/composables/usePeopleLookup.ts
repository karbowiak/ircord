import type { components } from '../../shared/generated/api'
import {
  createPeopleSearch,
  type PeopleScope,
  type PeopleLookupState,
} from '../../shared/utils/people-lookup'
import { peopleLookupCache } from '../utils/people-cache'

type Person = components['schemas']['Person']
type Context = Pick<PeopleScope, 'networkId' | 'purpose' | 'conversationId'> & { limit?: number }

/** Each picker owns its requests; the bounded cache is isolated by account and network. */
export function usePeopleLookup(context: () => Context) {
  const auth = useAuthStore()
  const state = shallowRef<PeopleLookupState<Person>>({ loading: false })
  const query = ref<string | null>(null)
  const searcher = createPeopleSearch(
    peopleLookupCache,
    ({ accountId: _, ...params }, signal) => apiClient().people(params, signal),
    (next) => {
      state.value = next
    },
  )
  function run() {
    const accountId = auth.account?.id ?? ''
    const current = context()
    searcher.search(
      query.value === null
        ? null
        : {
            ...current,
            accountId,
            query: query.value.trim(),
            limit: Math.max(1, Math.min(current.limit ?? 20, 50)),
          },
    )
  }
  function search(value: string | null) {
    query.value = value
    run()
  }
  watch(
    () => auth.account?.id,
    () => {
      searcher.cancel()
      peopleLookupCache.clear()
      state.value = { loading: false }
      run()
    },
    { flush: 'sync' },
  )
  watch(
    () => {
      const current = context()
      return [current.networkId, current.purpose, current.conversationId, current.limit]
    },
    run,
    { flush: 'sync' },
  )
  onBeforeUnmount(() => searcher.cancel())
  return { state, search }
}
