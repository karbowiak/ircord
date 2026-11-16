/** Persist a coherent device checkpoint, including on mobile page suspension. */
import { offlineArchive } from '../../utils/offline-archive'
import { createCheckpointSchedule } from '../../utils/checkpoint-schedule'
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
  const schedule = createCheckpointSchedule(() => chat.saveCheckpoint(recent.value))
  watch(() => [chat.cursor, chat.activeId, chat.status, chat.checkpointReady], schedule.changed)
  const visibility = useDocumentVisibility()
  watch(visibility, (state) => state === 'hidden' && schedule.flush(), { flush: 'sync' })
  useEventListener(window, 'pagehide', schedule.flush)
  onBeforeUnmount(schedule.flush)
}
