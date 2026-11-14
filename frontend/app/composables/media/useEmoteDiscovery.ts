import type { Ref } from 'vue'
import type { components } from '../../../shared/generated/api'

// These fields follow the discovery contract; the generated API types are owned by the coordinator.
export type DiscoveryItem = components['schemas']['DiscoveryItem']
export type DiscoveryPage = components['schemas']['DiscoveryResult']

/** Keep one bounded page, cancel obsolete work, and ignore responses from older requests. */
export function useEmoteDiscovery(enabled: Ref<boolean>) {
  const query = ref('')
  const source = ref('')
  const animation = ref('all')
  const page = ref<DiscoveryPage>()
  const loading = ref(false)
  const error = ref('')
  const cursors = ref<string[]>([''])
  let generation = 0
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined

  async function load() {
    const current = ++generation
    controller?.abort()
    if (!enabled.value) {
      loading.value = false
      return
    }
    controller = new AbortController()
    loading.value = true
    error.value = ''
    try {
      const result = await apiClient().discoverEmotes(
        {
          source: source.value || undefined,
          query: query.value.trim() || undefined,
          animation: animation.value,
          cursor: cursors.value.at(-1) || undefined,
          limit: 30,
        },
        controller.signal,
      )
      if (current === generation) page.value = result
    } catch (cause) {
      if (current === generation && !controller.signal.aborted) error.value = errorText(cause)
    } finally {
      if (current === generation) loading.value = false
    }
  }
  function refresh() {
    void load()
  }
  function restart() {
    cursors.value = ['']
    void load()
  }
  function next() {
    if (!page.value?.nextCursor || loading.value) return
    cursors.value = [...cursors.value, page.value.nextCursor]
    void load()
  }
  function previous() {
    if (cursors.value.length < 2 || loading.value) return
    cursors.value = cursors.value.slice(0, -1)
    void load()
  }
  watch(
    [enabled, query, source, animation],
    () => {
      generation++
      controller?.abort()
      clearTimeout(timer)
      cursors.value = ['']
      error.value = ''
      loading.value = enabled.value
      if (enabled.value)
        timer = setTimeout(() => {
          void load()
        }, 250)
    },
    { immediate: true },
  )
  onScopeDispose(() => {
    generation++
    controller?.abort()
    clearTimeout(timer)
  })
  return {
    query,
    source,
    animation,
    page,
    loading,
    error,
    cursors,
    refresh,
    restart,
    next,
    previous,
  }
}
