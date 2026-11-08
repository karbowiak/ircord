import { acceptHMRUpdate, defineStore } from 'pinia'
import { useQuery, useQueryCache } from '@pinia/colada'
import type { Message } from '../../shared/types/chat'
import { dropSavedRef, putSavedRef, savedChanged, savedIdsQuery } from '../queries/saved'

/** Whether a message can be bookmarked: delivered, real chat, not deleted. */
export const savable = (m: Message) =>
  !m.delivery && !m.deleted && (m.kind === 'message' || m.kind === 'action' || m.kind === 'notice')

/**
 * Your private saved messages. The server owns them; this store shows which messages are
 * saved and runs save/remove. A request in flight is its own pending state (“Saving…”): a
 * message only shows as saved once the server confirms it. Other devices follow through
 * saved.added / saved.removed events.
 */
export const useSavedStore = defineStore('saved', () => {
  const chat = useChatStore()
  const queryCache = useQueryCache()
  const refs = useQuery(() => ({ ...savedIdsQuery, enabled: !!chat.data }))
  const ids = computed(() => new Set(refs.data.value?.items.map((r) => r.messageId)))
  /** Requests in flight: the state each message is changing to. */
  const pending = reactive(new Map<string, boolean>())

  /** Confirmed by the server; never true just because a save is on its way. */
  const isSaved = (id: string) => ids.value.has(id)
  const isPending = (id: string) => pending.has(id)
  /** The change in flight for a message, if any. */
  const pendingChange = (id: string): 'saving' | 'removing' | undefined =>
    pending.has(id) ? (pending.get(id) ? 'saving' : 'removing') : undefined

  async function toggle(message: Message, want = !isSaved(message.id)) {
    if (!savable(message) && want) return chat.notify('Only delivered messages can be saved.')
    if (pending.has(message.id) || want === ids.value.has(message.id)) return
    if (navigator.onLine === false)
      return chat.notify(
        want
          ? 'You’re offline. Saving a message needs a connection.'
          : 'You’re offline. Removing a saved message needs a connection.',
      )
    pending.set(message.id, want)
    try {
      if (want) {
        const item = await apiClient().saveMessage(message.id, message.conversationId)
        putSavedRef(queryCache, item)
        savedChanged(queryCache)
      } else {
        await apiClient().unsaveMessage(message.id)
        dropSavedRef(queryCache, message.id)
      }
      chat.notify(want ? 'Saved for later.' : 'Removed from saved messages.')
    } catch (error) {
      chat.notify(
        `${want ? 'Couldn’t save the message' : 'Couldn’t remove the saved message'}: ${errorText(error)}`,
      )
    } finally {
      pending.delete(message.id)
    }
  }

  /** Removes a bookmark by id (from the saved list, where the source may be gone). */
  async function remove(messageId: string) {
    if (pending.has(messageId)) return
    pending.set(messageId, false)
    try {
      await apiClient().unsaveMessage(messageId)
      dropSavedRef(queryCache, messageId)
      chat.notify('Removed from saved messages.')
    } catch (error) {
      chat.notify(`Couldn’t remove the saved message: ${errorText(error)}`)
    } finally {
      pending.delete(messageId)
    }
  }

  return {
    refs,
    /** Whether the saved references have loaded, so “not in ids” means “not saved”. */
    loaded: computed(() => !!refs.data.value),
    ids,
    isSaved: computed(() => isSaved),
    isPending: computed(() => isPending),
    pendingChange: computed(() => pendingChange),
    toggle,
    remove,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useSavedStore, import.meta.hot))
