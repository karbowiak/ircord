import type { Conversation, Network } from '../../shared/types/chat'
import { channelTarget } from '../../shared/utils/irc'
import { conversationPath, isConversationPath } from '../../shared/utils/workspace-routes'
import { startupConversation } from '../../shared/utils/startup'
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
  const { t } = useI18n()
  const chat = useChatStore()
  const preferences = usePreferencesStore()
  const router = useRouter()
  let applying = false
  let initialized = false
  let revision = 0
  const initialSelection = chat.selectionRevision
  const initialPreferenceRevision = preferences.revision
  const initialAddress = router.currentRoute.value.fullPath
  let disposed = false
  let finishWaiting: (() => void) | undefined
  onScopeDispose(() => {
    disposed = true
    finishWaiting?.()
    chat.finishStartup()
  })

  /** Wait briefly for the account preference; a cached, account-scoped copy works offline. */
  function waitForPreferences() {
    if (
      preferences.ready ||
      preferences.query.error ||
      !navigator.onLine ||
      preferences.revision !== initialPreferenceRevision ||
      chat.selectionRevision !== initialSelection ||
      router.currentRoute.value.fullPath !== initialAddress
    )
      return Promise.resolve()
    return new Promise<void>((resolve) => {
      const stop = watch(
        [
          () => preferences.ready,
          () => preferences.query.error,
          () => preferences.revision,
          () => chat.selectionRevision,
          () => router.currentRoute.value.fullPath,
        ],
        ([ready, error, preferenceRevision, selection, address]) => {
          if (
            ready ||
            error ||
            preferenceRevision !== initialPreferenceRevision ||
            selection !== initialSelection ||
            address !== initialAddress
          )
            finish()
        },
      )
      const timer = setTimeout(() => finish(), 1500)
      function finish() {
        clearTimeout(timer)
        stop()
        finishWaiting = undefined
        resolve()
      }
      finishWaiting = finish
    })
  }

  async function legacy(hash: string) {
    const value = decodeURIComponent(hash.replace(/^#/, ''))
    const slash = value.indexOf('/')
    const slug = (slash < 0 ? value : value.slice(0, slash)).toLowerCase()
    const target = slash < 0 ? '' : value.slice(slash + 1)
    const names = slugs(chat.data?.networks ?? [])
    const network = chat.data?.networks.find((n) => names.get(n.id) === slug || n.id === slug)
    if (!network) throw new Error(t('workspaceUi.commands.networkNotAdded'))
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
        if (!person) throw new Error(t('workspaceUi.commands.personNotAround', { nick }))
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
      const notification =
        route.path === '/' && typeof route.query.c === 'string' ? route.query.c : ''
      if (notification) {
        if (!chat.data.conversations.some((conversation) => conversation.id === notification))
          throw new Error(t('workspaceUi.commands.conversationClosed'))
        chat.activate(notification)
      } else if (route.path === '/' && route.hash) await legacy(route.hash)
      else {
        const id =
          'id' in route.params && typeof route.params.id === 'string' ? route.params.id : ''
        if (route.path.startsWith('/server')) {
          const network = id ? chat.data.networks.find((n) => n.id === id) : chat.network
          if (network) chat.activate(serverViewId(network.id))
          else if (id) throw new Error(t('workspaceUi.commands.networkGone'))
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
          else if (id) throw new Error(t('workspaceUi.commands.conversationClosed'))
          else return // The route shows its empty state until a conversation is selected.
        }
      }
      if (current !== revision) return
      const path = conversationPath(chat.activeId, chat.data.conversations)
      if (path && (route.path !== path || route.hash)) await router.replace(path)
    } catch (error) {
      chat.notify(error instanceof Error ? error.message : errorText(error, t))
      if (!chat.activeId) {
        const id = startupConversation(
          chat.data,
          { mode: 'last', channelId: null },
          chat.lastConversation,
        )
        if (id) chat.activate(id)
      }
      const fallback = conversationPath(chat.activeId, chat.data.conversations)
      if (fallback && current === revision) await router.replace(fallback)
    } finally {
      await nextTick()
      if (current === revision) applying = false
    }
  }

  async function initialize() {
    const route = router.currentRoute.value
    const explicit =
      (route.path === '/' && (!!route.hash || !!route.query.c)) ||
      /^\/(channel|dm|server)(\/|$)/.test(route.path)
    initialized = true
    if (!explicit) {
      await waitForPreferences()
      if (disposed) return
      if (
        chat.selectionRevision !== initialSelection ||
        router.currentRoute.value.fullPath !== initialAddress
      ) {
        chat.finishStartup()
        return
      }
      if (chat.data) {
        const id = startupConversation(
          chat.data,
          preferences.revision === initialPreferenceRevision
            ? preferences.preferences.startup
            : { mode: 'last', channelId: null },
          chat.lastConversation,
        )
        if (id) {
          applying = true
          chat.activate(id, isConversationPath(route.path))
          applying = false
        }
      }
    }
    chat.finishStartup()
    if (!disposed) await apply()
  }

  const stopInitial = watch(
    () => chat.data,
    (data) => {
      if (!data) return
      queueMicrotask(() => stopInitial())
      void initialize()
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
    { flush: 'sync' },
  )

  const safari = /^((?!chrome|chromium|android|crios|fxios|edg).)*safari/i.test(navigator.userAgent)
  function mouseButtons(event: MouseEvent) {
    if (event.button !== 3 && event.button !== 4) return
    event.preventDefault()
    if (event.button === 3) router.back()
    else router.forward()
  }
  if (safari) useEventListener(window, 'mouseup', mouseButtons)
}
