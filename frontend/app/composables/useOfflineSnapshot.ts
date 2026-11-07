import { useQueryCache } from '@pinia/colada'
import type { Bootstrap, MessagePage } from '../../shared/types/chat'
import { bootstrapQuery, messagesQuery } from '../queries/chat'

const CONVERSATIONS = 5
const MESSAGES = 100

interface Snapshot {
  savedAt: string
  bootstrap: Bootstrap
  messages: Record<string, MessagePage>
}

/**
 * Keeps a small copy of the workspace (the snapshot plus recent history of the last few
 * conversations) on this device. When the server can't be reached at start-up, that copy is
 * shown read-only instead of an error, and the socket reconnects and resyncs once it can.
 * Call once from the workspace.
 */
export function useOfflineSnapshot() {
  const chat = useChatStore()
  const auth = useAuthStore()
  const queryCache = useQueryCache()
  const { recent } = useRecentConversations()
  const key = computed(() => (auth.account ? `ircord.snapshot.${auth.account.id}` : ''))

  let timer: ReturnType<typeof setTimeout> | undefined
  function save() {
    clearTimeout(timer)
    timer = setTimeout(() => {
      const bootstrap = chat.data
      if (!key.value || !bootstrap || chat.status === 'offline') return
      const ids = [chat.activeId, ...recent.value].filter((id, i, all) => all.indexOf(id) === i)
      const messages: Record<string, MessagePage> = {}
      for (const id of ids) {
        if (Object.keys(messages).length >= CONVERSATIONS) break
        const page = queryCache.getQueryData<MessagePage>(messagesQuery(id).key)
        if (page) messages[id] = { items: page.items.slice(-MESSAGES), nextCursor: page.nextCursor }
      }
      // Storage full or unavailable: the app works without a snapshot.
      writeLocal(key.value, {
        savedAt: new Date().toISOString(),
        bootstrap,
        messages,
      } satisfies Snapshot)
    }, 3000)
  }
  watch(() => [chat.data, chat.activeId, chat.outbox[chat.activeId]?.length], save, { deep: false })
  const stopMessages = queryCache.$onAction?.(({ name, after }) => {
    if (name === 'setQueryData') after(save)
  })

  // Start-up failed because the network is unreachable (not a server refusal): use the copy.
  watch(
    () => chat.bootstrap.error,
    (error) => {
      const status = (error as { statusCode?: number } | null)?.statusCode
      if (!error || chat.data || (status && status < 500) || !key.value) return
      const snapshot = readLocal<Snapshot | null>(key.value, (v) =>
        v && typeof v === 'object' && 'bootstrap' in v ? (v as Snapshot) : null,
      )
      if (!snapshot) return
      for (const [id, page] of Object.entries(snapshot.messages))
        queryCache.setQueryData(messagesQuery(id).key, page)
      queryCache.setQueryData(bootstrapQuery.key, snapshot.bootstrap)
      chat.notify(
        `Offline: showing your workspace as of ${new Date(snapshot.savedAt).toLocaleString()}.`,
      )
    },
  )
  onBeforeUnmount(() => {
    clearTimeout(timer)
    stopMessages?.()
  })
}
