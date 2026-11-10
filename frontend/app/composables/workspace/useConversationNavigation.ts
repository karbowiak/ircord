import { conversationPath } from '../../../shared/utils/workspace-routes'

/** Navigate through Nuxt so every destination participates in browser history. */
export function useConversationNavigation() {
  const router = useRouter()
  const chat = useChatStore()
  const pathFor = (id: string) => conversationPath(id, chat.data?.conversations ?? [])
  async function open(id: string) {
    const path = pathFor(id)
    if (path) await router.push(path)
  }
  return { open, current: () => open(chat.activeId), pathFor }
}
