/** Persist a coherent device checkpoint, including on mobile page suspension. */
import { offlineArchive } from '../utils/offline-archive'
export function useOfflineSnapshot() {
  const { t, locale } = useI18n()
  offlineArchive.setTranslator((key) => t(key))
  watch(locale, () => offlineArchive.refreshError())
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
  const save = () => {
    stop()
    chat.saveCheckpoint(recent.value)
  }
  const { start: schedule, stop } = useTimeoutFn(save, 1000, { immediate: false })
  watch(() => [chat.cursor, chat.activeId, chat.status, chat.checkpointReady], schedule)
  const visibility = useDocumentVisibility()
  watch(visibility, (state) => state === 'hidden' && save(), { flush: 'sync' })
  useEventListener(window, 'pagehide', save)
  onBeforeUnmount(save)
}
