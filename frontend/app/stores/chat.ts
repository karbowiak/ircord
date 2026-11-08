import { normalizeTransport, realtimeUrl } from '../utils/transport'
import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import type {
  Bootstrap,
  ClientEvent,
  Conversation,
  Member,
  Message,
  MessagePage,
  Network,
  NetworkInput,
  ProfileUpdate,
  ServerEvent,
  ServerLogPage,
  Upload,
} from '../../shared/types/chat'
import type { GifItem } from '../../shared/types/media'
import {
  messageActionReason as actionReason,
  conversationSendReason,
  messageTextReason,
  type MessageAction,
} from '../../shared/utils/features'
import { markdownToIrc } from '../../shared/utils/irc'
import { selfForNetwork } from '../../shared/utils/identity'
import {
  bootstrapQuery,
  HISTORY_PAGE,
  MAX_MESSAGES,
  messagesQuery,
  serverLogQuery,
} from '../queries/chat'

/** Conversation ids of the form `server:<networkId>` open that network's status window. */
export const serverViewId = (networkId: string) => `server:${networkId}`
const noMembers: Member[] = []

export type ConnectionStatus = 'connecting' | 'connected' | 'reconnecting' | 'offline'

const sanitizeDrafts = (value: unknown) => {
  const drafts: Record<string, string> = {}
  if (value && typeof value === 'object')
    for (const [id, text] of Object.entries(value).slice(0, 100))
      if (typeof text === 'string' && text) drafts[id] = text.slice(0, 4000)
  return drafts
}

/**
 * Workspace state: the bootstrap snapshot and message pages live in the Pinia Colada cache; this
 * store owns the realtime socket that keeps them current, plus UI state (active conversation,
 * drafts, unsent messages).
 */
export const useChatStore = defineStore('chat', () => {
  const config = useRuntimeConfig().public
  const appUpdate = useAppUpdate()
  appUpdate.attach()
  const auth = useAuthStore()
  const accountPrefix = `ircord.${auth.account?.id ?? 'signed-out'}`
  const queryCache = useQueryCache()
  const bootstrap = useQuery(bootstrapQuery)
  const data = computed(() => bootstrap.data.value)

  const activeId = ref(
    readLocal(`${accountPrefix}.active`, (v) => (typeof v === 'string' ? v : 'general'), 'session'),
  )
  const networkId = ref('')
  const status = ref<ConnectionStatus>('connecting')
  const notice = ref('')
  // Drafts survive closing the tab (localStorage), scoped to the account.
  const drafts = reactive(readLocal(`${accountPrefix}.drafts`, sanitizeDrafts, 'local'))
  /** Messages sent from this tab that the server has not acknowledged yet, per conversation. */
  const outbox = reactive<Record<string, Message[]>>({})
  const loadingOlder = ref(false)
  /** Unread count at the moment a conversation was opened; places the “new messages” line. */
  const unreadMarkers = reactive<Record<string, number>>({})
  /** Text another component wants inserted into the composer (mentions from menus). */
  const pendingInsert = ref('')
  const incomingListeners = new Set<(message: Message, mention: boolean) => void>()

  /** The network whose status window is open, or '' when a conversation is open. */
  const serverView = computed(() =>
    activeId.value.startsWith('server:') ? activeId.value.slice('server:'.length) : '',
  )
  const active = computed(() => data.value?.conversations.find((c) => c.id === activeId.value && c.networkId === networkId.value))
  const network = computed(() => data.value?.networks.find((n) => n.id === networkId.value))
  /**
   * The open conversation's member ids, kept as the same array while the content is unchanged:
   * unread counts and server updates replace the conversation (and its id list) far more often
   * than anyone joins or leaves, and roster views redo their work for every new array.
   */
  const activeMemberIds = computed<string[]>((previous) => {
    const ids = active.value?.memberIds ?? []
    return previous && previous.length === ids.length && previous.every((id, i) => id === ids[i])
      ? previous
      : ids
  })
  const self = computed(() =>
    selfForNetwork(data.value?.user, network.value?.nick, network.value?.realname),
  )
  // The snapshot is replaced on every unread count or conversation change; reading members
  // through this keeps the lookup maps (thousands of entries in busy channels) from being
  // rebuilt unless the member list itself changed.
  const members = computed(() => data.value?.members ?? noMembers)
  const memberMap = computed(() => new Map(members.value.map((m) => [m.id, m])))
  const member = (id: string): Member | undefined => id === 'you' ? self.value : memberMap.value.get(id)
  const nickMap = computed(
    () => new Map(members.value.map((m) => [m.nick.toLowerCase(), m])),
  )
  const memberByNick = (nick: string): Member | undefined => nickMap.value.get(nick.toLowerCase())
  const directPartner = (c: Conversation) => member(c.memberIds.find((id) => id !== 'you') ?? '')
  const sendReason = computed(() => {
    if (serverView.value)
      return status.value !== 'connected' || network.value?.status !== 'connected'
        ? 'Reconnect to this network to send commands.'
        : undefined
    return conversationSendReason(active.value, network.value, status.value === 'connected')
  })
  const canSend = computed(() => !sendReason.value)
  const networkOf = (c: Pick<Conversation, 'networkId'>) =>
    data.value?.networks.find((n) => n.id === c.networkId)

  const messageActionReason = (message: Message, action: MessageAction) => {
    const conversation = data.value?.conversations.find((c) => c.id === message.conversationId)
    return actionReason(
      message,
      conversation,
      conversation && networkOf(conversation),
      action,
      status.value === 'connected',
    )
  }
  const messageActionSupported = (message: Message, action: MessageAction) => {
    const conversation = data.value?.conversations.find((c) => c.id === message.conversationId)
    return !!(conversation && networkOf(conversation)?.features[action])
  }

  let noticeTimer: ReturnType<typeof setTimeout> | undefined
  function notify(text: string) {
    clearTimeout(noticeTimer)
    notice.value = text
    noticeTimer = setTimeout(() => (notice.value = ''), 4500)
  }

  // ---- Cache updates -------------------------------------------------------------------------

  function patchBootstrap(change: (value: Bootstrap) => Bootstrap) {
    const current = queryCache.getQueryData(bootstrapQuery.key)
    if (current) queryCache.setQueryData(bootstrapQuery.key, change(current))
  }
  const upsert = <T extends { id: string }>(list: T[], item: T) =>
    list.some((x) => x.id === item.id)
      ? list.map((x) => (x.id === item.id ? item : x))
      : [...list, item]
  function putConversation(c: Conversation) {
    const first = !data.value?.conversations.some((item) => item.networkId === c.networkId)
    patchBootstrap((b) => ({ ...b, conversations: upsert(b.conversations, c) }))
    if (!active.value && (!serverView.value || first) && (!networkId.value || networkId.value === c.networkId))
      activate(c.id)
  }
  function putNetwork(n: Network) {
    const previous = data.value?.networks.find((item) => item.id === n.id)
    patchBootstrap((b) => ({ ...b, networks: upsert(b.networks, n) }))
    if (
      previous &&
      (previous.host !== n.host || previous.port !== n.port || previous.status !== n.status)
    )
      void queryCache.invalidateQueries({ key: ['channels', n.id] })
  }
  /** A conversation or network went away; leave it if it was open. */
  function leaveRemoved(isGone: (c: Conversation) => boolean) {
    const gone = active.value && isGone(active.value)
    patchBootstrap((b) => ({ ...b, conversations: b.conversations.filter((c) => !isGone(c)) }))
    if (gone) {
      const next =
        data.value?.conversations.find((c) => c.networkId === networkId.value) ??
        data.value?.conversations[0]
      if (next) activate(next.id)
    }
  }
  function removeNetworkLocally(id: string) {
    leaveRemoved((c) => c.networkId === id)
    patchBootstrap((b) => ({ ...b, networks: b.networks.filter((n) => n.id !== id), invitations: (b.invitations ?? []).filter(v => v.networkId !== id) }))
    if (networkId.value === id || serverView.value === id) {
      const next = data.value?.networks[0]
      if (next) selectNetwork(next.id)
      else {
        activeId.value = ''
        networkId.value = ''
        writeLocal(`${accountPrefix}.active`, '', 'session')
      }
    }
  }
  const putProfile = (m: Member) =>
    patchBootstrap((b) => ({
      ...b,
      user: m,
      members: b.members.map((x) => (x.id === m.id ? m : x)),
    }))

  function putMessage(message: Message) {
    if (message.clientId) settle(message.conversationId, message.clientId)
    const key = messagesQuery(message.conversationId).key
    const page = queryCache.getQueryData<MessagePage>(key)
    // Conversations that are not cached load fresh history when opened.
    if (page)
      queryCache.setQueryData(key, {
        ...page,
        items: upsert(page.items, message).slice(-MAX_MESSAGES),
      })
  }

  // ---- Outbox ---------------------------------------------------------------------------------

  const pendingTimers = new Map<string, ReturnType<typeof setTimeout>>()
  function settle(conversationId: string, clientId: string) {
    clearTimeout(pendingTimers.get(clientId))
    pendingTimers.delete(clientId)
    attachmentIds.delete(clientId)
    const list = outbox[conversationId]
    if (list) outbox[conversationId] = list.filter((m) => m.clientId !== clientId)
  }
  function markFailed(clientId: string) {
    clearTimeout(pendingTimers.get(clientId))
    pendingTimers.delete(clientId)
    for (const [id, list] of Object.entries(outbox))
      if (list.some((m) => m.clientId === clientId))
        outbox[id] = list.map((m) => (m.clientId === clientId ? { ...m, delivery: 'failed' } : m))
  }

  // ---- Realtime -------------------------------------------------------------------------------

  let socket: WebSocket | undefined
  let cursor = 0
  let attempts = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined
  let readTimer: ReturnType<typeof setTimeout> | undefined
  let resyncing: Promise<void> | undefined

  // ---- Typing ---------------------------------------------------------------------------------

  /** conversationId → ids of people typing there. Entries expire, as IRCv3 `+typing` suggests. */
  const typing = reactive<Record<string, string[]>>({})
  const typingTimers = new Map<string, ReturnType<typeof setTimeout>>()
  function setTyping({
    conversationId,
    memberId,
    state,
  }: {
    conversationId: string
    memberId: string
    state: string
  }) {
    if (memberId === 'you') return
    const key = `${conversationId} ${memberId}`
    clearTimeout(typingTimers.get(key))
    typingTimers.delete(key)
    const others = (typing[conversationId] ?? []).filter((id) => id !== memberId)
    if (state === 'active') {
      others.push(memberId)
      typingTimers.set(
        key,
        setTimeout(() => setTyping({ conversationId, memberId, state: 'done' }), 6000),
      )
    }
    if (others.length) typing[conversationId] = others
    else delete typing[conversationId]
  }
  let typingSentAt = 0
  let typingSentTo = ''
  /** Tells the others you're typing (at most every 3s), or that you stopped. */
  let typingIdle: ReturnType<typeof setTimeout> | undefined
  /**
   * Tells the others you're typing (at most every 3s), paused (5s without keystrokes) or done.
   * Stopping always goes to the conversation that was told, even after switching away.
   */
  function sendTyping(state: 'active' | 'paused' | 'done') {
    if (!socket || socket.readyState !== WebSocket.OPEN) return
    const frame = (conversationId: string, value: 'active' | 'paused' | 'done') => {
      const conversation = data.value?.conversations.find((c) => c.id === conversationId)
      const network = conversation && networkOf(conversation)
      if (
        !network?.features.typing ||
        network.status !== 'connected' ||
        !!conversationSendReason(conversation, network)
      )
        return
      socket?.send(
        JSON.stringify({
          v: 1,
          type: 'typing.set',
          conversationId,
          state: value,
        } satisfies ClientEvent),
      )
    }
    clearTimeout(typingIdle)
    if (state !== 'active') {
      if (typingSentTo) frame(typingSentTo, state)
      typingSentAt = 0
      if (state === 'done') typingSentTo = ''
      return
    }
    if (serverView.value || !activeId.value) return
    if (typingSentTo && typingSentTo !== activeId.value) frame(typingSentTo, 'done')
    typingIdle = setTimeout(() => sendTyping('paused'), 5000)
    const now = Date.now()
    if (typingSentTo === activeId.value && now - typingSentAt < 3000) return
    typingSentAt = now
    typingSentTo = activeId.value
    frame(activeId.value, 'active')
  }
  watch(activeId, () => sendTyping('done'))

  // Presence and profile changes arrive in bursts in big channels (dozens a second). Applying
  // each one copies the member list and rebuilds the lookup maps, so they're merged in one pass
  // every 50ms instead.
  const pendingMembers = new Map<string, Member>()
  let memberFlush: ReturnType<typeof setTimeout> | undefined
  function queueMemberUpdate(updated: Member) {
    pendingMembers.set(updated.id, updated)
    memberFlush ??= setTimeout(flushMembers, 50)
  }
  function flushMembers() {
    clearTimeout(memberFlush)
    memberFlush = undefined
    if (!pendingMembers.size) return
    const updates = new Map(pendingMembers)
    pendingMembers.clear()
    patchBootstrap((b) => {
      const merged = b.members.map((m) => {
        const next = updates.get(m.id)
        updates.delete(m.id)
        return next ?? m
      })
      return { ...b, members: updates.size ? [...merged, ...updates.values()] : merged }
    })
  }

  function applyEvent(event: ServerEvent) {
    if (event.v !== 1) return
    if (event.type === 'session.ready') {
      socket?.send(JSON.stringify({ v: 1, type: 'session.resume', after: cursor }))
      status.value = 'connected'
      attempts = 0
      // A release restarts the frontend and drops this socket, so a reconnect may mean a new build.
      void appUpdate.check()
      return
    }
    if (event.type === 'session.resync') return void resync()
    if (event.type === 'typing') return setTyping(event.data)
    if (event.type === 'command.error') {
      if (event.clientId) markFailed(event.clientId)
      return notify(event.message)
    }
    if (event.seq <= cursor) {
      // A retried command is acknowledged again with its original sequence number.
      if (event.type === 'message.created' && event.data.clientId) putMessage(event.data)
      return
    }
    if (event.seq > cursor + 1) return void resync()
    cursor = event.seq
    switch (event.type) {
      case 'message.created':
      case 'message.updated':
        putMessage(event.data)
        if (event.type === 'message.created')
          setTyping({ ...event.data, memberId: event.data.senderId, state: 'done' })
        if (
          event.type === 'message.created' &&
          event.data.senderId !== 'you' &&
          !event.data.historical
        )
          countUnread(event.data)
        break
      case 'conversation.updated':
        putConversation(event.data)
        if (
          !data.value?.demo &&
          event.data.unreadCount &&
          readingActiveConversation(event.data.id)
        ) {
          clearTimeout(readTimer)
          readTimer = setTimeout(() => {
            if (readingActiveConversation(event.data.id)) void markRead(event.data.id)
          }, 300)
        }
        break
      case 'invitation.received':
        patchBootstrap(b => ({ ...b, invitations: upsert(b.invitations ?? [], event.data) }))
        break
      case 'invitation.removed':
        patchBootstrap(b => ({ ...b, invitations: (b.invitations ?? []).filter(v => v.id !== event.data.id) }))
        break
      case 'network.updated':
        putNetwork(event.data)
        break
      case 'connection.updated': {
        const key = ['connection', event.data.networkId]
        // An in-flight HTTP snapshot must not overwrite a newer live handshake.
        queryCache.cancelQueries({ key, exact: true })
        queryCache.setQueryData(key, event.data)
        break
      }
      case 'notification-settings.updated':
        void queryCache.invalidateQueries({ key: ['notification-settings'] })
        break
      case 'appearance.updated':
      case 'library.updated':
      case 'client-preferences.updated':
        void queryCache.invalidateQueries({
          key: [
            event.type === 'client-preferences.updated' ? 'preferences' : event.type.split('.')[0]!,
          ],
        })
        break
      case 'member.updated':
        queueMemberUpdate(event.data)
        break
      case 'history.synced':
        void queryCache.invalidateQueries({ key: ['messages', event.data.conversationId] })
        break
      case 'history.gap':
      case 'history.sync.failed':
        notify(
          event.data.reason ?? event.data.message ?? 'Message history is temporarily unavailable.',
        )
        break
      case 'profile.updated':
        putProfile(event.data)
        break
      case 'conversation.removed':
        leaveRemoved((c) => c.id === event.data.id)
        break
      case 'network.removed':
        removeNetworkLocally(event.data.id)
        break
      case 'server-log.updated':
        void queryCache.invalidateQueries({ key: ['server-log', event.data.networkId] })
        break
      case 'server.line': {
        const key = serverLogQuery(event.data.networkId).key
        const page = queryCache.getQueryData<ServerLogPage>(key)
        if (page)
          queryCache.setQueryData(key, { ...page, items: [...page.items, event.data].slice(-1000) })
        break
      }
    }
  }
  function countUnread(message: Message) {
    const c = data.value?.conversations.find((x) => x.id === message.conversationId)
    if (!c || message.kind === 'event') return
    const notifications = useNotificationStore()
    if (notifications.ignored(member(message.senderId)?.nick)) return
    // IRC servers don't count mentions; the client does: your nick on that network or a highlight word.
    const mention =
      c.kind === 'direct' ||
      notifications.highlights(message.text, networkOf(c)?.nick ?? data.value?.user.nick)
    if (data.value?.demo)
      putConversation({
        ...c,
        unreadCount: c.unreadCount + 1,
        mentionCount: c.mentionCount + (mention && c.kind === 'channel' ? 1 : 0),
      })
    incomingListeners.forEach((listener) => listener(message, mention))
    if (readingActiveConversation(message.conversationId)) {
      clearTimeout(readTimer)
      readTimer = setTimeout(() => {
        if (readingActiveConversation(message.conversationId)) void markRead(message.conversationId)
      }, 300)
    }
  }
  function connect() {
    clearTimeout(reconnectTimer)
    if (socket) {
      socket.onclose = null
      socket.close()
    }
    const url = realtimeUrl(config.apiBase, config.websocketUrl, location)
    status.value = attempts ? 'reconnecting' : 'connecting'
    socket = new WebSocket(url)
    socket.onmessage = (frame) => {
      try {
        applyEvent(data.value?.demo ? normalizeTransport(JSON.parse(frame.data)) : parseServerEvent(frame.data))
      } catch {
        notify('An update could not be read. Reconnect to refresh.')
      }
    }
    socket.onerror = () => (status.value = 'reconnecting')
    socket.onclose = async (event) => {
      if (!data.value?.demo) {
        try {
          const session = await apiClient().session()
          if (!session.account) {
            auth.expire()
            return
          }
        } catch {
          /* Reconnect with backoff after network errors. */
        }
      }
      status.value = navigator.onLine ? 'reconnecting' : 'offline'
      if (event.code === 1008) return void resync().then(connect)
      attempts++
      reconnectTimer = setTimeout(
        () => (navigator.onLine ? connect() : (status.value = 'offline')),
        Math.min(30_000, 800 * 2 ** Math.min(attempts, 5)) + Math.random() * 300,
      )
    }
  }
  function reconnect() {
    attempts = 0
    connect()
  }
  /** Replaces the snapshot and drops cached history, which may have gaps. */
  function resync() {
    resyncing ??= (async () => {
      try {
        await bootstrap.refetch()
        if (data.value) cursor = data.value.cursor
        queryCache.invalidateQueries({ key: ['messages'] })
        queryCache.invalidateQueries({ key: ['connection'] })
      } catch (error) {
        notify(errorText(error))
      } finally {
        resyncing = undefined
      }
    })()
    return resyncing
  }

  // First snapshot: pick the network and open the socket.
  // Runs once, the first time there is data. A flag, not the stop handle: with a cached snapshot
  // the immediate call runs before watch() has returned it.
  let ready = false
  watch(
    () => data.value,
    (value) => {
      if (!value || ready) return
      ready = true
      cursor = value.cursor
      const server = activeId.value.startsWith('server:') ? activeId.value.slice(7) : ''
      if (server && value.networks.some((n) => n.id === server)) networkId.value = server
      else {
        if (!value.conversations.some((c) => c.id === activeId.value))
          activeId.value = value.conversations[0]?.id ?? ''
        networkId.value =
          value.conversations.find((c) => c.id === activeId.value)?.networkId ??
          value.networks[0]?.id ??
          ''
      }
      connect()
      if (readingActiveConversation(activeId.value)) void markRead(activeId.value)
    },
    { immediate: true },
  )

  // ---- Navigation -----------------------------------------------------------------------------

  function readingActiveConversation(id: string) {
    return activeId.value === id && document.visibilityState === 'visible' && document.hasFocus()
  }

  async function markRead(id: string) {
    const c = data.value?.conversations.find((x) => x.id === id)
    if (!c?.unreadCount && !c?.mentionCount) return
    const optimistic = { ...c, unreadCount: 0, mentionCount: 0 }
    putConversation(optimistic)
    const projected = data.value?.conversations.find((item) => item.id === id)
    try {
      const saved = await apiClient().updateConversation(id, { read: true })
      // Live events own unread state. An older HTTP response must not erase a
      // newer message received after this command was committed.
      if (data.value?.demo && data.value.conversations.find((item) => item.id === id) === projected)
        putConversation(saved)
    } catch {
      // Roll back only our own projection, never a newer canonical live update.
      if (data.value?.conversations.find((item) => item.id === id) === projected)
        putConversation(c)
      notify('Couldn’t mark this conversation read. Try again.')
    }
  }
  function activate(id: string) {
    if (id.startsWith('server:')) {
      const target = id.slice('server:'.length)
      if (!data.value?.networks.some((n) => n.id === target)) return
      activeId.value = id
      networkId.value = target
      writeLocal(`${accountPrefix}.active`, id, 'session')
      return
    }
    const c = data.value?.conversations.find((x) => x.id === id)
    if (!c) return
    if (c.unreadCount) unreadMarkers[id] = c.unreadCount
    else delete unreadMarkers[id]
    activeId.value = id
    networkId.value = c.networkId
    writeLocal(`${accountPrefix}.active`, id, 'session')
    if (readingActiveConversation(id)) void markRead(id)
  }
  function selectNetwork(id: string) {
    if (!data.value?.networks.some((n) => n.id === id)) return
    const c = data.value?.conversations.find((x) => x.networkId === id && x.kind === 'channel')
      ?? data.value?.conversations.find((x) => x.networkId === id)
    activate(c?.id ?? serverViewId(id))
  }
  function step(direction: 1 | -1) {
    const list = data.value?.conversations.filter((c) => c.networkId === networkId.value) ?? []
    const i = list.findIndex((c) => c.id === activeId.value)
    const next = list[(i + direction + list.length) % list.length]
    if (next) activate(next.id)
  }

  async function loadOlder(id: string) {
    const key = messagesQuery(id).key
    const page = queryCache.getQueryData<MessagePage>(key)
    if (!page?.nextCursor || loadingOlder.value) return
    loadingOlder.value = true
    try {
      const older = await apiClient().messages(id, { limit: HISTORY_PAGE, before: page.nextCursor })
      const current = queryCache.getQueryData<MessagePage>(key) ?? page
      const known = new Set(current.items.map((m) => m.id))
      queryCache.setQueryData(key, {
        items: [...older.items.filter((m) => !known.has(m.id)), ...current.items].slice(
          0,
          MAX_MESSAGES,
        ),
        nextCursor: older.nextCursor,
      })
    } catch (error) {
      notify(errorText(error))
    } finally {
      loadingOlder.value = false
    }
  }

  // ---- Sending --------------------------------------------------------------------------------

  function transmit(message: Message) {
    const clientId = message.clientId!
    const payload = {
      clientId,
      text: message.text,
      ...(message.replyTo ? { replyToId: message.replyTo.id } : {}),
      ...(message.gif ? { gif: message.gif } : {}),
      ...(message.kind === 'action' ? { kind: 'action' as const } : {}),
      ...(attachmentIds.get(clientId) ? { attachments: attachmentIds.get(clientId) } : {}),
    }
    pendingTimers.set(
      clientId,
      setTimeout(() => void reconcile(message), 15_000),
    )
    if (!data.value?.demo) {
      void apiClient().sendMessage(message.conversationId, payload)
        .then((accepted) => {
          if (accepted.message) putMessage(accepted.message)
        })
        .catch((error) => {
          markFailed(clientId)
          notify(errorText(error))
        })
      return
    }
    if (socket?.readyState !== WebSocket.OPEN) return markFailed(clientId)
    socket.send(
      JSON.stringify({
        v: 1,
        type: 'message.send',
        conversationId: message.conversationId,
        ...payload,
      }),
    )
  }
  async function reconcile(message: Message) {
    if (data.value?.demo) return markFailed(message.clientId!)
    try {
      const command = await apiClient().command(message.clientId!)
      if (command.message) {
        putMessage(command.message)
        return
      }
      if (command.state === 'uncertain')
        notify('Delivery is uncertain. Refresh the conversation before sending again.')
    } catch {
      /* The durable command remains available when connectivity returns. */
    }
    markFailed(message.clientId!)
  }
  const attachmentIds = new Map<string, string[]>()
  /**
   * Sends to the open conversation. Markdown-style shortcuts become IRC formatting codes, so
   * other IRC clients see bold and italics too.
   */
  function send(
    text: string,
    replyTo?: Message,
    gif?: GifItem,
    options: { kind?: 'message' | 'action'; upload?: Upload } = {},
  ) {
    if (!canSend.value || !active.value) {
      notify(sendReason.value ?? 'Choose a conversation before sending a message.')
      return false
    }
    if (!text.trim()) return false
    const wireText = gif || options.upload ? text.trim() : markdownToIrc(text.trim())
    const reason =
      (replyTo && messageActionReason(replyTo, 'replies')) ||
      (network.value &&
        messageTextReason(wireText, options.kind ?? 'message', active.value, network.value))
    if (reason) {
      notify(reason)
      return false
    }
    const clientId = crypto.randomUUID()
    if (options.upload) attachmentIds.set(clientId, [options.upload.id])
    delete unreadMarkers[activeId.value]
    const message: Message = {
      id: `pending-${clientId}`,
      clientId,
      conversationId: activeId.value,
      senderId: 'you',
      text: wireText,
      createdAt: new Date().toISOString(),
      kind: options.kind ?? 'message',
      reactions: [],
      delivery: 'pending',
      ...(gif ? { gif } : {}),
      ...(options.upload
        ? {
            attachment: {
              name: options.upload.name,
              url: options.upload.url,
              width: options.upload.width ?? 0,
              height: options.upload.height ?? 0,
              alt: options.upload.name,
              contentType: options.upload.contentType,
              size: options.upload.size,
            },
          }
        : {}),
      ...(replyTo
        ? {
            replyTo: {
              id: replyTo.id,
              nick: member(replyTo.senderId)?.nick ?? '',
              text: replyTo.text.slice(0, 160),
            },
          }
        : {}),
    }
    outbox[message.conversationId] = [...(outbox[message.conversationId] ?? []), message]
    transmit(message)
    return true
  }
  function retry(message: Message) {
    const list = outbox[message.conversationId] ?? []
    outbox[message.conversationId] = list.map((m) =>
      m.clientId === message.clientId ? { ...m, delivery: 'pending' } : m,
    )
    transmit(message)
  }
  function discard(message: Message) {
    if (message.clientId) settle(message.conversationId, message.clientId)
  }
  /** Sends one IRC line to a network. Failures arrive as `command.error` notices. */
  function raw(line: string, target = networkId.value) {
    if (
      socket?.readyState !== WebSocket.OPEN ||
      data.value?.networks.find((n) => n.id === target)?.status !== 'connected'
    ) {
      notify('Not connected. Try again in a moment.')
      return false
    }
    if (!data.value?.demo) {
      void apiClient()
        .raw(target, { clientId: crypto.randomUUID(), line })
        .catch((error) => notify(errorText(error)))
      return true
    }
    socket.send(
      JSON.stringify({
        v: 1,
        type: 'raw.send',
        clientId: crypto.randomUUID(),
        networkId: target,
        line,
      }),
    )
    return true
  }
  function onIncoming(listener: (message: Message, mention: boolean) => void) {
    incomingListeners.add(listener)
    return () => incomingListeners.delete(listener)
  }

  // ---- Mutations ------------------------------------------------------------------------------

  async function react(message: Message, emoji: string) {
    const reason = messageActionReason(message, 'reactions')
    if (reason) return notify(reason)
    try {
      putMessage(await apiClient().react(message.conversationId, { messageId: message.id, emoji }))
    } catch (error) {
      notify(errorText(error))
    }
  }

  /**
   * Deletes one of your messages for everyone. The timeline changes only once the
   * server reports that the network accepted it. Resolves with the file-retention
   * report, or undefined after showing the failure.
   */
  async function redact(message: Message) {
    const reason = messageActionReason(message, 'redact')
    if (reason) {
      notify(reason)
      return
    }
    try {
      const result = await apiClient().redact(message.conversationId, message.id)
      putMessage(result.message)
      return result
    } catch (error) {
      notify(errorText(error))
      return undefined
    }
  }
  /** Deletes an account-owned upload; returns whether it succeeded. */
  async function deleteUpload(id: string) {
    try {
      await apiClient().deleteUpload(id)
      return true
    } catch (error) {
      notify(errorText(error))
      return false
    }
  }

  const favorite = useMutation({
    mutation: (c: Conversation) =>
      apiClient().updateConversation(c.id, { favorite: !c.favorite }),
    onMutate: (c) => putConversation({ ...c, favorite: !c.favorite }),
    onSuccess: putConversation,
    onError: (error, c) => {
      putConversation(c)
      notify(errorText(error))
    },
  })

  /** Who fetches link previews here: this browser (default) or the backend. */
  const previewMode = useMutation({
    mutation: ({ c, mode }: { c: Conversation; mode: 'client' | 'server' }) =>
      apiClient().updateConversation(c.id, { previewMode: mode }),
    onMutate: ({ c, mode }) => putConversation({ ...c, previewMode: mode }),
    onSuccess: putConversation,
    onError: (error, { c }) => {
      putConversation(c)
      notify(errorText(error))
    },
  })
  const join = useMutation({
    mutation: (input: string | { name: string; key?: string; networkId?: string }) => {
      const value = typeof input === 'string' ? { name: input } : input
      return apiClient().createConversation({ networkId: value.networkId ?? networkId.value, name: value.name, key: value.key })
    },
    onSuccess: (c) => {
      // JOIN acceptance can reach HTTP after the newer roster/joined event.
      // The socket owns live state; only seed a conversation that is still absent.
      if (!data.value?.conversations.some(existing => existing.id === c.id)) putConversation(c)
      activate(c.id)
    },
  })
  const rejectInvitation = useMutation({
    mutation: (id: string) => apiClient().rejectInvitation(id),
    onSuccess: (_, id) => patchBootstrap(b => ({ ...b, invitations: (b.invitations ?? []).filter(v => v.id !== id) })),
  })
  const direct = useMutation({
    mutation: (memberId: string) =>
      apiClient().createConversation({ networkId: networkId.value, memberId }),
    onSuccess: (c) => {
      putConversation(c)
      activate(c.id)
    },
  })
  /** Leaves a channel (PART) or closes a direct conversation. */
  const leave = useMutation({
    mutation: ({ c, reason }: { c: Conversation; reason?: string }) =>
      apiClient().leaveConversation(c.id, reason),
    onSuccess: (_result, { c }) => leaveRemoved((x) => x.id === c.id),
    onError: (error) => notify(errorText(error)),
  })
  const addNetwork = useMutation({
    mutation: async (input: NetworkInput) => {
      const n = await apiClient().createNetwork(input)
      putNetwork(n)
      // Join the auto-join channels, or a first channel so the network isn't empty.
      const channels = data.value?.demo
        ? input.autoJoin.length
          ? input.autoJoin
          : ['#general']
        : []
      const joined: Conversation[] = []
      for (const entry of channels) {
        const [name, key] = entry.split(' ')
        joined.push(
          await apiClient().createConversation({ networkId: n.id, name, key }),
        )
      }
      return { network: n, conversations: joined }
    },
    onSuccess: ({ network: n, conversations }) => {
      conversations.forEach(putConversation)
      if (conversations[0]) activate(conversations[0].id)
      else selectNetwork(n.id)
      notify(`Added ${n.name}.`)
    },
  })
  const updateNetwork = useMutation({
    mutation: ({ id, input }: { id: string; input: NetworkInput }) =>
      apiClient().updateNetwork(id, input),
    onSuccess: putNetwork,
  })
  const removeNetwork = useMutation({
    mutation: (n: Network) => apiClient().deleteNetwork(n.id),
    onSuccess: (_result, n) => {
      removeNetworkLocally(n.id)
      notify(`Removed ${n.name}.`)
    },
    onError: (error) => notify(errorText(error)),
  })
  const connection = useMutation({
    mutation: (n: Network) =>
      apiClient().updateNetwork(n.id, { status: n.status === 'connected' || n.status === 'connecting' ? 'disconnected' : 'connected' }),
    onSuccess: putNetwork,
    onError: (error) => notify(errorText(error)),
  })
  const profile = useMutation({
    mutation: (input: ProfileUpdate) => apiClient().profile(input),
    onSuccess: putProfile,
  })

  watch(networkId, async (id) => {
    if (!id || data.value?.demo) return
    patchBootstrap((b) => ({ ...b, capabilities: { ...b.capabilities, upload: undefined } }))
    try {
      const provider = await apiClient().fileProvider(id)
      if (networkId.value !== id) return
      patchBootstrap((b) => ({
        ...b,
        capabilities: {
          ...b.capabilities,
          upload: provider.available
            ? { maxBytes: provider.maxBytes, types: provider.types ?? [] }
            : undefined,
        },
      }))
    } catch {
      /* Uploading stays unavailable until provider discovery succeeds. */
    }
  })

  // ---- Browser lifecycle ----------------------------------------------------------------------

  let draftTimer: ReturnType<typeof setTimeout> | undefined
  /** Writes drafts now instead of after the debounce, e.g. right before a reload. */
  function flushDrafts() {
    clearTimeout(draftTimer)
    writeLocal(`${accountPrefix}.drafts`, drafts, 'local')
  }
  watch(drafts, () => {
    clearTimeout(draftTimer)
    draftTimer = setTimeout(flushDrafts, 200)
  })
  const onOnline = () => reconnect()
  const onOffline = () => (status.value = 'offline')
  const onVisible = () => {
    if (readingActiveConversation(activeId.value)) void markRead(activeId.value)
  }
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
  window.addEventListener('focus', onVisible)
  // Like the layout and library stores: keep keystrokes typed just before any reload or close.
  window.addEventListener('pagehide', flushDrafts)
  document.addEventListener('visibilitychange', onVisible)
  function dispose() {
    clearTimeout(memberFlush)
    pendingMembers.clear()
    clearTimeout(reconnectTimer)
    clearTimeout(noticeTimer)
    clearTimeout(readTimer)
    clearTimeout(draftTimer)
    pendingTimers.forEach(clearTimeout)
    if (socket) {
      socket.onclose = null
      socket.close()
    }
    window.removeEventListener('online', onOnline)
    window.removeEventListener('offline', onOffline)
    window.removeEventListener('focus', onVisible)
    window.removeEventListener('pagehide', flushDrafts)
    appUpdate.detach()
    document.removeEventListener('visibilitychange', onVisible)
  }

  return {
    bootstrap,
    data,
    self,
    activeId,
    networkId,
    status,
    notice,
    drafts,
    flushDrafts,
    outbox,
    loadingOlder,
    unreadMarkers,
    pendingInsert,
    serverView,
    active,
    network,
    canSend,
    sendReason,
    // Getters returning the function, not plain functions: Pinia wraps returned functions as
    // actions (argument copies, listener bookkeeping on every call), and these lookups run
    // thousands of times per update in big channels. Callers still write chat.member(id).
    activeMemberIds,
    member: computed(() => member),
    memberByNick: computed(() => memberByNick),
    directPartner: computed(() => directPartner),
    networkOf: computed(() => networkOf),
    messageActionReason: computed(() => messageActionReason),
    messageActionSupported: computed(() => messageActionSupported),
    notify,
    activate,
    selectNetwork,
    step,
    markRead,
    loadOlder,
    send,
    raw,
    retry,
    discard,
    onIncoming,
    react,
    redact,
    deleteUpload,
    favorite,
    previewMode,
    typing,
    sendTyping,
    join,
    rejectInvitation,
    leave,
    direct,
    addNetwork,
    updateNetwork,
    removeNetwork,
    connection,
    profile,
    reconnect,
    resync,
    dispose,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useChatStore, import.meta.hot))
