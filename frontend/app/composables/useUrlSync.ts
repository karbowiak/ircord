import type { Conversation, Network } from '../../shared/types/chat'
import { channelTarget } from '../../shared/utils/irc'
import { conversationPath, isConversationPath } from '../../shared/utils/workspace-routes'
import { serverViewId } from '../stores/chat'

/** Legacy hash links remain readable while new links use native Nuxt routes. */
function slugs(networks: Network[]) {
  const base = (n: Network) =>
    n.name
      .toLowerCase()
      .normalize('NFKD')
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '') || n.id
  const counts = new Map<string, number>()
  for (const n of networks) counts.set(base(n), (counts.get(base(n)) ?? 0) + 1)
  return new Map(
    networks.map((n) => [
      n.id,
      counts.get(base(n))! > 1 ? `${base(n)}-${n.id.slice(0, 6)}` : base(n),
    ]),
  )
}

/** One owner for route ↔ conversation selection. Nuxt owns browser history and page routing. */
export function useUrlSync() {
  const chat = useChatStore()
  const router = useRouter()
  let applying = false
  let initialized = false
  let revision = 0

  async function legacy(hash: string) {
    const value = decodeURIComponent(hash.replace(/^#/, ''))
    const slash = value.indexOf('/')
    const slug = (slash < 0 ? value : value.slice(0, slash)).toLowerCase()
    const target = slash < 0 ? '' : value.slice(slash + 1)
    const names = slugs(chat.data?.networks ?? [])
    const network = chat.data?.networks.find((n) => names.get(n.id) === slug || n.id === slug)
    if (!network) throw new Error('That link points to a network you haven’t added.')
    if (!target) return chat.activate(serverViewId(network.id))
    const inNetwork = (c: Conversation) => c.networkId === network.id
    if (/^[#&+!*~]/.test(target)) {
      const c = chat.data?.conversations.find(
        (c) =>
          inNetwork(c) &&
          c.kind === 'channel' &&
          channelTarget(c).toLowerCase() === target.toLowerCase(),
      )
      if (c) chat.activate(c.id)
      else {
        chat.selectNetwork(network.id)
        await chat.join.mutateAsync({ name: target, networkId: network.id })
      }
    } else if (target.startsWith('@')) {
      const nick = target.slice(1)
      const c = chat.data?.conversations.find(
        (c) =>
          inNetwork(c) &&
          c.kind === 'direct' &&
          chat.directPartner(c)?.nick.toLowerCase() === nick.toLowerCase(),
      )
      if (c) chat.activate(c.id)
      else {
        chat.selectNetwork(network.id)
        const person = chat.memberByNick(nick)
        if (!person) throw new Error(`No one called ${nick} is around.`)
        await chat.direct.mutateAsync(person.id)
      }
    }
  }

  async function apply() {
    const route = router.currentRoute.value
    if (!chat.data || !isConversationPath(route.path)) return
    const current = ++revision
    applying = true
    try {
      if (route.path === '/' && route.hash) await legacy(route.hash)
      else {
        const id = typeof route.params.id === 'string' ? route.params.id : ''
        if (route.path.startsWith('/server')) {
          const network = id ? chat.data.networks.find((n) => n.id === id) : chat.network
          if (network) chat.activate(serverViewId(network.id))
          else if (id) throw new Error('That network is no longer in your workspace.')
        } else if (route.path.startsWith('/channel') || route.path.startsWith('/dm')) {
          const kind = route.path.startsWith('/channel') ? 'channel' : 'direct'
          const conversation = id
            ? chat.data.conversations.find((c) => c.id === id && c.kind === kind)
            : chat.active?.kind === kind && !chat.serverView
              ? chat.active
              : chat.data.conversations.find(
                  (c) => c.kind === kind && c.networkId === chat.networkId,
                )
          if (conversation) chat.activate(conversation.id)
          else if (id) throw new Error('That conversation is no longer open.')
          else return // The route shows its empty state until a conversation is selected.
        }
      }
      if (current !== revision) return
      const path = conversationPath(chat.activeId, chat.data.conversations)
      if (path && (route.path !== path || route.hash)) await router.replace(path)
    } catch (error) {
      chat.notify(error instanceof Error ? error.message : errorText(error))
      const fallback = conversationPath(chat.activeId, chat.data.conversations)
      if (fallback && current === revision) await router.replace(fallback)
    } finally {
      await nextTick()
      if (current === revision) applying = false
    }
  }

  const stopInitial = watch(
    () => chat.data,
    (data) => {
      if (!data) return
      queueMicrotask(() => stopInitial())
      void apply().finally(() =>
        nextTick(() => {
          initialized = true
        }),
      )
    },
    { immediate: true },
  )
  watch(
    () => router.currentRoute.value.fullPath,
    () => {
      void apply()
    },
  )
  // Menus, commands and live account changes still select through the chat store.
  watch(
    () => chat.activeId,
    (id) => {
      if (!initialized || applying || !chat.data) return
      const path = conversationPath(id, chat.data.conversations)
      if (path && router.currentRoute.value.path !== path) void router.push(path)
    },
  )

  const safari = /^((?!chrome|chromium|android|crios|fxios|edg).)*safari/i.test(navigator.userAgent)
  function mouseButtons(event: MouseEvent) {
    if (event.button !== 3 && event.button !== 4) return
    event.preventDefault()
    if (event.button === 3) router.back()
    else router.forward()
  }
  if (safari) window.addEventListener('mouseup', mouseButtons)
  onBeforeUnmount(() => {
    if (safari) window.removeEventListener('mouseup', mouseButtons)
  })
}
