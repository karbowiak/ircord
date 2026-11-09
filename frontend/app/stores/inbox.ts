import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import { inboxChanged, inboxSummaryQuery } from '../queries/inbox'

/**
 * Your private attention inbox: mentions, replies to your messages and direct messages, kept by
 * the server whether or not a client was open. This store holds the unread indicator and runs
 * dismiss/restore. Dismissing only hides an item; it never marks its conversation read.
 */
export const useInboxStore = defineStore('inbox', () => {
  const { $i18n } = useNuxtApp()
  const chat = useChatStore()
  const queryCache = useQueryCache()
  const summary = useQuery(() => ({ ...inboxSummaryQuery, enabled: !!chat.data }))
  /** Requests in flight: whether each item is being dismissed (true) or restored (false). */
  const pending = reactive(new Map<string, boolean>())

  async function setDismissed(messageId: string, dismissed: boolean) {
    if (pending.has(messageId)) return
    if (navigator.onLine === false) return chat.notify($i18n.t('workspaceUi.stores.inboxOffline'))
    pending.set(messageId, dismissed)
    try {
      await (dismissed ? apiClient().dismissInbox(messageId) : apiClient().restoreInbox(messageId))
      inboxChanged(queryCache, 0)
    } catch (error) {
      chat.notify(
        $i18n.t(
          dismissed ? 'workspaceUi.stores.dismissFailed' : 'workspaceUi.stores.restoreFailed',
          { error: errorText(error, (key) => $i18n.t(key)) },
        ),
      )
    } finally {
      pending.delete(messageId)
    }
  }

  return {
    summary,
    unread: computed(() => summary.data.value?.unread ?? 0),
    isPending: computed(() => (id: string) => pending.has(id)),
    dismiss: (id: string) => setDismissed(id, true),
    restore: (id: string) => setDismissed(id, false),
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useInboxStore, import.meta.hot))
