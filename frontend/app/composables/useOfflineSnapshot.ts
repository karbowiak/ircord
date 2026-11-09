/** Persist a coherent device checkpoint, including on mobile page suspension. */
import { offlineArchive } from '../utils/offline-archive'
export function useOfflineSnapshot() {
  const chat = useChatStore()
  const auth = useAuthStore()
  const config = useRuntimeConfig().public
  watch(
    () => chat.status,
    (status) => offlineArchive.connection(status),
    { immediate: true },
  )
  watch(
    () => [auth.account?.id, chat.data?.demo, !!chat.data] as const,
    ([owner, demo, loaded]) => {
      if (owner && loaded && !demo) void offlineArchive.start(owner, config.apiBase)
    },
    { immediate: true, flush: 'sync' },
  )
  const { recent } = useRecentConversations()
  let timer: ReturnType<typeof setTimeout> | undefined
  const save = () => {
    clearTimeout(timer)
    chat.saveCheckpoint(recent.value)
  }
  const schedule = () => {
    clearTimeout(timer)
    timer = setTimeout(save, 1000)
  }
  watch(() => [chat.cursor, chat.activeId, chat.status, chat.checkpointReady], schedule)
  const hide = () => {
    if (document.visibilityState === 'hidden') save()
  }
  document.addEventListener('visibilitychange', hide)
  window.addEventListener('pagehide', save)
  onBeforeUnmount(() => {
    save()
    document.removeEventListener('visibilitychange', hide)
    window.removeEventListener('pagehide', save)
  })
}
