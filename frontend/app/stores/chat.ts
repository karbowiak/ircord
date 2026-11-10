import { recipientIn } from '../../shared/group-mentions'
import { isConversationPath } from '../../shared/utils/workspace-routes'
import { normalizeTransport, realtimeUrl, setTransportSelf } from '../utils/transport'
import { acceptHMRUpdate, defineStore } from 'pinia'
import { useMutation, useQuery, useQueryCache } from '@pinia/colada'
import type {
  Bootstrap,
  ChannelSettingsChange,
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
import type { components } from '../../shared/generated/api'
import { ATTRACT_MS, type AttractTarget } from '../../shared/utils/attract'
import {
  messageActionReason as actionReason,
  messageActionReasonKey as actionReasonKey,
  conversationSendReason,
  messageTextReason,
  type MessageAction,
} from '../../shared/utils/features'
import { markdownToIrc, wireText as toWireText } from '../../shared/utils/irc'
import { selfForNetwork } from '../../shared/utils/identity'
import {
  discardDraft as discardDraftText,
  mergeDrafts,
  restoreDraft as restoreDraftText,
  sanitizeDrafts,
  sanitizeDraftTimes,
  stampDraftTimes,
} from '../../shared/utils/drafts'
import {
  bootstrapQuery,
  HISTORY_PAGE,
  MAX_MESSAGES,
  messagesQuery,
  serverLogQuery,
} from '../queries/chat'
import { dropSavedRef, isSavedRef, patchSaved, putSavedRef, savedChanged } from '../queries/saved'
import { devicesQuery } from '../queries/devices'
import { inboxChanged, inboxRead } from '../queries/inbox'
import { linksChanged, linksMessage } from '../queries/links'
import {
  mergeMessagePage,
  newestMessagePage,
  updateMessagePage,
} from '../../shared/utils/message-page'
import { readCheckpoint, writeCheckpoint } from '../utils/workspace-checkpoint'
import { offlineArchive, readMessages } from '../utils/offline-archive'
import { settleHistoryRefresh } from '../utils/history-refresh'
import { createHistoryPagination } from '../utils/history-pagination'
import { boundedRead, createRealtimeRecovery, RECOVERY } from '../utils/realtime-recovery'
type NotificationAlert = components['schemas']['Alert']

/** Conversation ids of the form `server:<networkId>` open that network's status window. */
export const serverViewId = (networkId: string) => `server:${networkId}`
const noMembers: Member[] = []

export type ConnectionStatus = 'connecting' | 'syncing' | 'connected' | 'reconnecting' | 'offline'

/**
 * Workspace state: the bootstrap snapshot and message pages live in the Pinia Colada cache; this
 * store owns the realtime socket that keeps them current, plus UI state (active conversation,
 * drafts, unsent messages).
 */
export const useChatStore = defineStore('chat', () => {
  const { $i18n } = useNuxtApp()
  const translate = (key: string, values?: Record<string, string | number>) =>
    values
      ? $i18n.t(
          key,
          Object.fromEntries(
            Object.entries(values).map(([name, value]) => [
              name,
              typeof value === 'number' ? $i18n.n(value) : value,
            ]),
          ),
        )
      : $i18n.t(key)
  const config = useRuntimeConfig().public
  const appUpdate = useAppUpdate()
  appUpdate.attach()
  const auth = useAuthStore()
  setTransportSelf(auth.account?.id)
  const accountPrefix = `ircord.${auth.account?.id ?? 'signed-out'}`
  const queryCache = useQueryCache()
  const checkpoint = auth.account && readCheckpoint(auth.account.id)
  if (checkpoint) {
    for (const [id, page] of Object.entries(checkpoint.messages)) {
      const key = messagesQuery(id).key
      queryCache.setQueryData(key, { ...page, partial: true })
      // A persisted display snapshot cannot prove that newer history is still contiguous.
      // Render it while the first active query refreshes the authoritative newest page.
      void queryCache.invalidateQueries({ key, exact: true }, false)
    }
    queryCache.setQueryData(bootstrapQuery.key, checkpoint.bootstrap)
  }
  const bootstrap = useQuery(bootstrapQuery)
  const data = computed(() => bootstrap.data.value)

  const savedConversation = readLocal(
    `${accountPrefix}.last-conversation`,
    (v) => (typeof v === 'string' ? v : ''),
  )
  // Migrate the old tab-scoped selection only when this account has no durable selection.
  const sessionConversation = readLocal(
    `${accountPrefix}.active`,
    (v) => (typeof v === 'string' && !v.startsWith('server:') ? v : ''),
    'session',
  )
  const lastConversation = savedConversation || sessionConversation
  const activeId = ref('')
  const selectionRevision = ref(0)
  let startupPending = true
  function finishStartup() {
    startupPending = false
  }
  const networkId = ref('')
  const status = ref<ConnectionStatus>('connecting')
  const notice = ref('')
  // Drafts survive closing the tab (localStorage), scoped to the account.
  const drafts = reactive(readLocal(`${accountPrefix}.drafts`, (v) => sanitizeDrafts(v), 'local'))
  // Last-edit times live beside the drafts (same payload as before), for the Drafts overview.
  // Drafts saved before times were kept stay without one rather than getting a made-up time.
  const draftTimes = reactive(
    readLocal(`${accountPrefix}.draft-times`, (v) => sanitizeDraftTimes(v, drafts), 'local'),
  )
  /** False once this device refused to save drafts (storage full or blocked). */
  const draftsSaved = ref(true)
  /** Messages sent from this tab that the server has not acknowledged yet, per conversation. */
  const outbox = reactive<Record<string, Message[]>>({})
  const loadingOlder = ref(false)
  const loadingNewer = ref(false)
  let aroundRead: AbortController | undefined
  const pagination = createHistoryPagination({
    pageSize: HISTORY_PAGE,
    limit: MAX_MESSAGES,
    get: (id) => queryCache.getQueryData<MessagePage>(messagesQuery(id).key),
    set: (id, page) => queryCache.setQueryData(messagesQuery(id).key, page),
    read: readMessages,
    changed: (direction) => {
      loadingOlder.value = direction === 'older'
      loadingNewer.value = direction === 'newer'
    },
    report: (error) => notifyError(error),
  })
  function cancelHistoryReads() {
    pagination.cancel()
    aroundRead?.abort()
    aroundRead = undefined
  }
  watch(activeId, cancelHistoryReads, { flush: 'sync' })
  /** Unread count at the moment a conversation was opened; places the “new messages” line. */
  const unreadMarkers = reactive<Record<string, number>>({})
  /** A manual “mark as unread” boundary to place that line at, per conversation. */
  const unreadAnchors = reactive<Record<string, string>>({})
  /** Conversations with a “mark as unread” still on its way to the server. */
  const unreadPending = new Set<string>()
  /** Text another component wants inserted into the composer (mentions from menus). */
  const pendingInsert = ref('')
  const incomingListeners = new Set<(message: Message, mention: boolean) => void>()
  const alertListeners = new Set<(alert: NotificationAlert) => void>()
  /** The server routes alerts across devices; set from session.ready. */
  const routing = ref(false)
  /** This tab's ID, pairing its activity reports and routed alerts with its socket. */
  const tabId = crypto.randomUUID()

  /** The network whose status window is open, or '' when a conversation is open. */
  const serverView = computed(() =>
    activeId.value.startsWith('server:') ? activeId.value.slice('server:'.length) : '',
  )
  const active = computed(() =>
    data.value?.conversations.find(
      (c) => c.id === activeId.value && c.networkId === networkId.value,
    ),
  )
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
  const member = (id: string): Member | undefined =>
    id === 'you' ? self.value : memberMap.value.get(id)
  const nickMap = computed(() => new Map(members.value.map((m) => [m.nick.toLowerCase(), m])))
  const memberByNick = (nick: string): Member | undefined => nickMap.value.get(nick.toLowerCase())
  const directPartner = (c: Conversation) => member(c.memberIds.find((id) => id !== 'you') ?? '')
  const sendReason = computed(() => {
    if (serverView.value)
      return status.value !== 'connected' || network.value?.status !== 'connected'
        ? translate('workspaceUi.chat.reconnectCommands')
        : undefined
    return conversationSendReason(
      active.value,
      network.value,
      status.value === 'connected',
      translate,
    )
  })
  const canSend = computed(() => !sendReason.value)

  const networkOf = (c: Pick<Conversation, 'networkId'>) =>
    data.value?.networks.find((n) => n.id === c.networkId)
  /** You as a conversation sees you, for highlights and group mentions in `message`. */
  const recipientFor = (c: Conversation, message?: Pick<Message, 'senderId' | 'historical'>) =>
    recipientIn(c, networkOf(c), data.value?.user, message)
  /** Like `canSend`, for any conversation (a lookup can finish after the user moved on). */
  const canSendTo = (c: Conversation | undefined) =>
    !!c && !conversationSendReason(c, networkOf(c), status.value === 'connected', translate)

  const messageActionReason = (message: Message, action: MessageAction) => {
    const conversation = data.value?.conversations.find((c) => c.id === message.conversationId)
    return actionReason(
      message,
      conversation,
      conversation && networkOf(conversation),
      action,
      status.value === 'connected',
      translate,
    )
  }
  const messageActionReasonKey = (message: Message, action: MessageAction) => {
    const conversation = data.value?.conversations.find((c) => c.id === message.conversationId)
    return actionReasonKey(
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

  // The toast stays long enough to read (longer for long text) and pauses while it is hovered or
  // focused. Errors stay until dismissed or replaced.
  const noticeError = ref(false)
  let noticeHeld = false
  let noticeTimer: ReturnType<typeof setTimeout> | undefined
  function notify(text: string, options: { error?: boolean } = {}) {
    notice.value = text
    noticeError.value = !!options.error
    scheduleNotice()
  }
  const notifyError = (error: unknown) => notify(errorText(error, translate), { error: true })
  function scheduleNotice() {
    clearTimeout(noticeTimer)
    if (!notice.value || noticeHeld || noticeError.value) return
    const visible = Math.max(6000, notice.value.length * 80)
    noticeTimer = setTimeout(() => (notice.value = ''), visible)
  }
  function holdNotice(hold: boolean) {
    noticeHeld = hold
    scheduleNotice()
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
    if (
      !startupPending &&
      !active.value &&
      (!serverView.value || first) &&
      (!networkId.value || networkId.value === c.networkId)
    )
      activate(c.id)
  }
  function putNetwork(n: Network) {
    const previous = data.value?.networks.find((item) => item.id === n.id)
    patchBootstrap((b) => ({ ...b, networks: upsert(b.networks, n) }))
    if (previous && (previous.account !== n.account || previous.status !== n.status)) {
      for (const c of data.value?.conversations.filter((c) => c.networkId === n.id) ?? []) {
        void queryCache
          .invalidateQueries({ key: ['channel-registration', c.id], exact: true })
          .catch(() => {})
        void queryCache
          .invalidateQueries({ key: ['channel-access', c.id], exact: true })
          .catch(() => {})
      }
    }
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
    patchBootstrap((b) => ({
      ...b,
      networks: b.networks.filter((n) => n.id !== id),
      invitations: (b.invitations ?? []).filter((v) => v.networkId !== id),
    }))
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

  function putMessage(message: Message, seq = 0) {
    if (message.clientId) settle(message.conversationId, message.clientId)
    const key = messagesQuery(message.conversationId).key
    let page = queryCache.getQueryData<MessagePage>(key)
    if (
      !page &&
      queryCache
        .getEntries({ key, exact: true })
        .some((entry) => entry.asyncStatus.value === 'loading')
    )
      page = { items: [], nextCursor: null, partial: true }
    // Conversations that are not cached load fresh history when opened. An older window
    // takes edits to retained messages, but never appends across its missing newer history.
    if (page) queryCache.setQueryData(key, updateMessagePage(page, message, seq))
  }

  const cachedMessage = (message: Message) =>
    queryCache
      .getQueryData<MessagePage>(messagesQuery(message.conversationId).key)
      ?.items.find((item) => item.id === message.id)
  // HTTP acknowledgments cannot replace a socket update received while waiting.
  function putMessageReply(message: Message, previous?: Message) {
    if (message.clientId) settle(message.conversationId, message.clientId)
    if (cachedMessage(message) === previous) putMessage(message)
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
  const cursor = ref(0)
  const pendingReads = ref(0)
  const checkpointTrusted = ref(true)
  const catchingUp = new Map<string, string>()
  // The live connection to IRCord, for the server page: since when, how often it was re-established
  // this session, and when the last update arrived.
  const liveSince = ref<number | null>(null)
  const reconnects = ref(0)
  const lastEventAt = ref<number | null>(null)
  let connectedOnce = false
  let readTimer: ReturnType<typeof setTimeout> | undefined
  let disposed = false
  const recoveryFailed = ref(false)
  const historyFailedId = ref('')
  const historyRefreshingId = ref('')
  const historyProblem = computed(() => !!activeId.value && historyFailedId.value === activeId.value)
  const historyRefreshing = computed(() => !!activeId.value && historyRefreshingId.value === activeId.value)
  let historyRefresh: { id: string; controller: AbortController; promise: Promise<void> } | undefined
  let historyChangedDuringReplay = false

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

  // ---- Attract --------------------------------------------------------------------------------

  /**
   * conversationId → the message /attract is pointing at, for `ATTRACT_MS`. Ephemeral like
   * typing: it lives only in memory and a reload forgets it. Your own attract plays here too.
   */
  const attention = reactive<Record<string, AttractTarget & { startedAt: number }>>({})
  const attentionTimers = new Map<string, ReturnType<typeof setTimeout>>()
  function setAttract({
    conversationId,
    memberId,
    messageId,
  }: components['schemas']['AttractEvent']['data']) {
    clearTimeout(attentionTimers.get(conversationId))
    const play = () => {
      attention[conversationId] = { memberId, messageId, startedAt: Date.now() }
      attentionTimers.set(
        conversationId,
        setTimeout(() => {
          attentionTimers.delete(conversationId)
          delete attention[conversationId]
        }, ATTRACT_MS),
      )
    }
    if (!attention[conversationId]) return play()
    // A new attract while one plays restarts it: render a frame without it first, so the
    // animations start over instead of carrying on.
    delete attention[conversationId]
    attentionTimers.set(conversationId, setTimeout(play, 0))
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
        !!conversationSendReason(conversation, network, true, translate)
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

  function refreshEmotes(version: number) {
    const loaded = queryCache.getQueryData<{ version: number }>(['emoji', 'instance'])
    if (loaded && loaded.version !== version)
      void queryCache.invalidateQueries({ key: ['emoji', 'instance'] }).catch(() => {})
  }

  function applyEvent(event: ServerEvent) {
    if (event.v !== 1) return
    lastEventAt.value = Date.now()
    if (event.type === 'session.ready') {
      catchingUp.clear()
      socket?.send(
        JSON.stringify({
          v: 1,
          type: 'session.resume',
          after: cursor.value,
          tab: tabId,
          sync: true,
        }),
      )
      recovery.ready(event.sync === true)
      routing.value = event.routing === true
      if (event.capabilities) patchBootstrap((b) => ({ ...b, capabilities: event.capabilities! }))
      // The shared emote catalog may have changed while this socket was away.
      if (event.emoteVersion !== undefined) refreshEmotes(event.emoteVersion)
      // A release restarts the frontend and drops this socket, so a reconnect may mean a new build.
      void appUpdate.check()
      return
    }
    if (event.type === 'session.synced') {
      if (event.cursor !== cursor.value) return void resync()
      flushMembers()
      recovery.synced()
      if (catchingUp.size) {
        const messages = catchingUp.size
        const conversations = new Set(catchingUp.values()).size
        notify(
          translate('workspaceUi.chat.caughtUp', {
            messages: $i18n.t(
              'workspaceUi.chat.newMessages',
              { count: $i18n.n(messages) },
              messages,
            ),
            conversations: $i18n.t(
              'workspaceUi.chat.conversations',
              { count: $i18n.n(conversations) },
              conversations,
            ),
          }),
        )
      }
      catchingUp.clear()
      return
    }
    if (event.type === 'session.resync') return void resync()
    if (event.type === 'typing') return setTyping(event.data)
    if (event.type === 'attract') return setAttract(event.data)
    if (event.type === 'emotes.updated') return refreshEmotes(event.data.version)
    // The server routed an alert to this device (#84): it decides when and where to interrupt.
    if (event.type === 'notification.alert') {
      for (const listener of alertListeners) listener(event.data)
      return
    }
    // A device signed in or out, or changed its push registration; an open list rereads.
    if (event.type === 'devices.updated')
      return void queryCache.invalidateQueries({ key: devicesQuery.key }).catch(() => {})
    if (event.type === 'command.error') {
      if (event.clientId) markFailed(event.clientId)
      return notify(event.message)
    }
    if (event.seq <= cursor.value) {
      // A retried command is acknowledged again with its original sequence number.
      if (event.type === 'message.created' && event.data.clientId) putMessage(event.data, event.seq)
      return
    }
    if (event.seq > cursor.value + 1) return void resync()
    switch (event.type) {
      case 'message.created':
      case 'message.updated':
        // The link catalog follows its sources (see linksMessage); it needs the copy before this event.
        linksMessage(queryCache, event.data, {
          updated: event.type === 'message.updated',
          channel:
            data.value?.conversations.find((c) => c.id === event.data.conversationId)?.kind ===
            'channel',
          ignored: useNotificationStore().ignored(member(event.data.senderId)?.nick),
          previousText: queryCache
            .getQueryData<MessagePage>(messagesQuery(event.data.conversationId).key)
            ?.items.find((m) => m.id === event.data.id)?.text,
        })
        putMessage(event.data, event.seq)
        if (
          status.value === 'syncing' &&
          event.type === 'message.created' &&
          event.data.senderId !== 'you' &&
          !event.data.deleted &&
          ['message', 'action', 'notice'].includes(event.data.kind)
        )
          catchingUp.set(event.data.id, event.data.conversationId)
        if (event.data.deleted) catchingUp.delete(event.data.id)
        // A saved message was edited or deleted: its excerpt follows the source.
        if (event.type === 'message.updated' && isSavedRef(queryCache, event.data.id))
          patchSaved(queryCache, event.data)
        // A deleted message drops its inbox excerpt and leaves the unread count.
        if (event.type === 'message.updated' && event.data.deleted) inboxChanged(queryCache)
        if (event.type === 'message.created')
          setTyping({ ...event.data, memberId: event.data.senderId, state: 'done' })
        if (
          event.type === 'message.created' &&
          event.data.senderId !== 'you' &&
          !event.data.historical &&
          status.value === 'connected'
        )
          countUnread(event.data)
        break
      case 'conversation.updated': {
        const previous = data.value?.conversations.find((c) => c.id === event.data.id)
        putConversation(event.data)
        // Reading a conversation (here or on another device) reads its inbox items.
        if (previous && event.data.unreadFrom !== previous.unreadFrom) inboxChanged(queryCache)
        else if (previous && event.data.unreadCount < previous.unreadCount) inboxRead(queryCache)
        if (previous?.memberModes?.you !== event.data.memberModes?.you) {
          void queryCache
            .invalidateQueries({ key: ['channel-registration', event.data.id], exact: true })
            .catch(() => {})
          void queryCache
            .invalidateQueries({ key: ['channel-access', event.data.id], exact: true })
            .catch(() => {})
        }
        readIfWatching(event.data.id, event.data.unreadCount)
        break
      }
      case 'conversation.counts': {
        // Only the unread state changed; the rest of the cached conversation stands.
        const previous = data.value?.conversations.find((c) => c.id === event.data.id)
        if (!previous) break
        const { unreadFrom, ...counts } = event.data
        const next = { ...previous, ...counts }
        if (unreadFrom) next.unreadFrom = unreadFrom
        else delete next.unreadFrom
        putConversation(next)
        if (unreadFrom !== previous.unreadFrom) inboxChanged(queryCache)
        else if (counts.unreadCount < previous.unreadCount) inboxRead(queryCache)
        readIfWatching(event.data.id, counts.unreadCount)
        break
      }
      case 'channel-services.updated':
        void queryCache
          .invalidateQueries({
            key: ['channel-registration', event.data.conversationId],
            exact: true,
          })
          .catch(() => {})
        void queryCache
          .invalidateQueries({ key: ['channel-access', event.data.conversationId], exact: true })
          .catch(() => {})
        break
      case 'invitation.received':
        patchBootstrap((b) => ({ ...b, invitations: upsert(b.invitations ?? [], event.data) }))
        break
      case 'invitation.removed':
        patchBootstrap((b) => ({
          ...b,
          invitations: (b.invitations ?? []).filter((v) => v.id !== event.data.id),
        }))
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
        // Ignoring someone hides their inbox items and their links.
        inboxChanged(queryCache)
        linksChanged(queryCache)
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
        if (event.data.conversationId === activeId.value) {
          if (status.value === 'connected') void refreshHistory()
          else historyChangedDuringReplay = true
        }
        else void queryCache.invalidateQueries({ key: ['messages', event.data.conversationId] }).catch(() => {})
        break
      case 'history.gap':
      case 'history.sync.failed':
        notify(
          event.data.reason ??
            event.data.message ??
            translate('workspaceUi.chat.historyUnavailable'),
        )
        break
      case 'profile.updated':
        putProfile(event.data)
        break
      case 'conversation.removed':
        leaveRemoved((c) => c.id === event.data.id)
        inboxChanged(queryCache)
        linksChanged(queryCache)
        break
      case 'network.removed':
        removeNetworkLocally(event.data.id)
        // The network's history went with it, and so did your bookmarks into it.
        void queryCache.invalidateQueries({ key: ['saved', 'ids'] }).catch(() => {})
        savedChanged(queryCache)
        inboxChanged(queryCache)
        linksChanged(queryCache)
        break
      case 'saved.added':
        putSavedRef(queryCache, event.data)
        savedChanged(queryCache)
        break
      case 'saved.removed':
        dropSavedRef(queryCache, event.data.messageId)
        break
      case 'inbox.added':
      case 'inbox.updated':
        inboxChanged(queryCache)
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
    cursor.value = event.seq
    recovery.progress()
    offlineArchive.event(event)
  }
  function countUnread(message: Message) {
    const c = data.value?.conversations.find((x) => x.id === message.conversationId)
    if (!c || message.kind === 'event') return
    const notifications = useNotificationStore()
    if (notifications.ignored(member(message.senderId)?.nick)) return
    // IRC servers don't count mentions; the client does: an @mention of your nick on that
    // network, a highlight word or a group mention that includes you.
    const mention =
      c.kind === 'direct' || notifications.highlights(message.text, recipientFor(c, message))
    if (data.value?.demo)
      putConversation({
        ...c,
        unreadCount: c.unreadCount + 1,
        mentionCount: c.mentionCount + (mention && c.kind === 'channel' ? 1 : 0),
      })
    incomingListeners.forEach((listener) => listener(message, mention))
    if (readingActiveConversation(message.conversationId)) scheduleAutoRead(message.conversationId)
  }
  function scheduleAutoRead(id: string) {
    clearTimeout(readTimer)
    readTimer = setTimeout(() => {
      if (readingActiveConversation(id)) void markRead(id, true)
    }, 300)
  }
  /** The server counted new unread messages in a conversation the user is looking at. */
  function readIfWatching(id: string, unreadCount: number) {
    if (
      !data.value?.demo &&
      status.value === 'connected' &&
      unreadCount &&
      readingActiveConversation(id)
    )
      scheduleAutoRead(id)
  }
  function cancelRecoveryHistory() {
    historyRefresh?.controller.abort()
    historyRefresh = undefined
    historyRefreshingId.value = ''
  }
  /** Refresh the live end without navigating or replacing an intentionally older window. */
  function refreshHistory(): Promise<void> {
    const id = activeId.value
    if (disposed || !id || id.startsWith('server:')) return Promise.resolve()
    if (historyRefresh?.id === id) return historyRefresh.promise
    cancelRecoveryHistory()
    const ticket = { id, controller: new AbortController(), promise: Promise.resolve() }
    historyRefresh = ticket
    historyRefreshingId.value = id
    ticket.promise = boundedRead(
      (signal) => readMessages(id, { limit: HISTORY_PAGE }, signal),
      ticket.controller,
      RECOVERY.history,
    ).then((page) => {
      if (disposed || historyRefresh !== ticket || ticket.controller.signal.aborted) return
      const key = messagesQuery(id).key
      const current = queryCache.getQueryData<MessagePage>(key)
      // Only an attached live window accepts a new newest page. Older reading anchors stay put.
      if ((!page.offline && !page.partial && !page.syncError || !current) && !current?.detached && !current?.hasNewer)
        queryCache.setQueryData(key, mergeMessagePage(page, current))
      if (page.offline || page.partial || page.syncError) historyFailedId.value = id
      else if (historyFailedId.value === id) historyFailedId.value = ''
    }).catch(() => {
      if (!disposed && historyRefresh === ticket) historyFailedId.value = id
    }).finally(() => {
      if (historyRefresh === ticket) {
        historyRefresh = undefined
        historyRefreshingId.value = ''
      }
    })
    return ticket.promise
  }
  watch(activeId, () => {
    cancelRecoveryHistory()
    historyFailedId.value = ''
  }, { flush: 'sync' })

  const recovery = createRealtimeRecovery<Bootstrap>({
    enabled: () => !disposed && !!data.value && (!!auth.account || !!data.value.demo),
    online: () => navigator.onLine,
    visible: () => document.visibilityState === 'visible',
    open: () => {
      const url = new URL(realtimeUrl(config.apiBase, config.websocketUrl, location))
      url.searchParams.set('sync', '1')
      return new WebSocket(url)
    },
    socket: (current) => { socket = current },
    receive: (raw) => applyEvent(data.value?.demo
      ? normalizeTransport(JSON.parse(raw))
      : parseServerEvent(raw)),
    snapshot: (signal) => apiClient().bootstrap(signal),
    invalidate: () => {
      flushMembers()
      cancelHistoryReads()
      cancelRecoveryHistory()
      checkpointTrusted.value = false
      queryCache.cancelQueries({ key: bootstrapQuery.key, exact: true })
      queryCache.cancelQueries({ key: ['messages'] })
    },
    install: (snapshot) => {
      // Keep readable messages while recovery runs; the next newest read reconciles their revisions.
      for (const entry of queryCache.getEntries({ key: ['messages'] })) {
        const page = entry.state.value.data as MessagePage | undefined
        if (page && !page.detached && !page.hasNewer)
          queryCache.setQueryData(entry.key, { ...page, partial: true })
      }
      queryCache.setQueryData(bootstrapQuery.key, snapshot)
      cursor.value = snapshot.cursor
      checkpointTrusted.value = true
      void queryCache.invalidateQueries({ key: ['messages'] }, false).catch(() => {})
      for (const key of ['connection', 'saved', 'inbox'])
        void queryCache.invalidateQueries({ key: [key] }).catch(() => {})
    },
    session: async (signal) => {
      if (data.value?.demo) return true
      try { return !!(await apiClient().session(signal)).account }
      catch (error) {
        if (errorStatus(error) === 401) return false
        throw error
      }
    },
    expired: () => auth.expire(),
    status: (value) => { status.value = value },
    failure: (value) => { recoveryFailed.value = value },
    lost: () => { liveSince.value = null },
    synced: (resumed) => {
      if (connectedOnce) reconnects.value++
      connectedOnce = true
      liveSince.value = Date.now()
      const activePage = queryCache.getQueryData<MessagePage>(messagesQuery(activeId.value).key)
      if (resumed || historyChangedDuringReplay || activePage?.offline || activePage?.partial) void refreshHistory()
      historyChangedDuringReplay = false
      // Other saved fallbacks become stale; opening them fetches an authoritative newest window.
      void queryCache.invalidateQueries({
        key: ['messages'],
        predicate: (entry) => {
          const page = entry.state.value.data as MessagePage | undefined
          return entry.key[1] !== activeId.value && (!!page?.offline || !!page?.partial)
        },
      }, false).catch(() => {})
    },
  })
  function reconnect() { recovery.reconnect() }
  function resync() { recovery.resync() }

  // First snapshot opens the socket; route sync resolves the initial selection separately.
  // Runs once, the first time there is data. A flag, not the stop handle: with a cached snapshot
  // the immediate call runs before watch() has returned it.
  let ready = false
  watch(
    () => data.value,
    (value) => {
      if (!value || ready) return
      ready = true
      cursor.value = value.cursor
      networkId.value = value.networks[0]?.id ?? ''
      recovery.start()
    },
    { immediate: true },
  )

  // Flush batched members before atomically saving state and its applied cursor.
  function saveCheckpoint(recent: string[]) {
    if (!auth.account || !data.value || data.value.demo || !checkpointReady.value)
      return
    flushMembers()
    const messages: Record<string, MessagePage> = {}
    for (const id of [activeId.value, ...recent]
      .filter((id, index, all) => all.indexOf(id) === index)
      .slice(0, 5)) {
      const page = queryCache.getQueryData<MessagePage>(messagesQuery(id).key)
      if (page && !page.partial && !page.detached && !page.hasNewer)
        messages[id] = newestMessagePage(page, 100)
    }
    writeCheckpoint({
      version: 3,
      accountId: auth.account.id,
      savedAt: new Date().toISOString(),
      bootstrap: { ...data.value, cursor: cursor.value },
      messages,
    })
  }

  // ---- Navigation -----------------------------------------------------------------------------

  function readingActiveConversation(id: string) {
    return (
      activeId.value === id &&
      isConversationPath(window.location.pathname) &&
      document.visibilityState === 'visible' &&
      document.hasFocus()
    )
  }

  /**
   * `auto` is reading by looking at the conversation (focus, new messages, reload). It
   * leaves a manual “mark as unread” alone, so that action isn't undone at once; reading
   * on purpose or switching to the conversation clears it.
   */
  async function markRead(id: string, auto = false) {
    const c = data.value?.conversations.find((x) => x.id === id)
    if (!c || (!c.unreadCount && !c.mentionCount && !c.unreadFrom)) return
    if (auto && (c.unreadFrom || unreadPending.has(id))) return
    pendingReads.value++
    const optimistic = { ...c, unreadCount: 0, mentionCount: 0, unreadFrom: undefined }
    putConversation(optimistic)
    const projected = data.value?.conversations.find((item) => item.id === id)
    try {
      const saved = await apiClient().updateConversation(id, { read: true })
      inboxRead(queryCache)
      // Live events own unread state. An older HTTP response must not erase a
      // newer message received after this command was committed.
      if (data.value?.demo && data.value.conversations.find((item) => item.id === id) === projected)
        putConversation(saved)
    } catch {
      // Roll back only our own projection, never a newer canonical live update.
      if (data.value?.conversations.find((item) => item.id === id) === projected) putConversation(c)
      notify(translate('workspaceUi.chat.markReadFailed'))
    } finally {
      pendingReads.value--
    }
  }
  /** Makes a message and everything after it unread again, on every device. */
  async function markUnread(message: Message) {
    const c = data.value?.conversations.find((x) => x.id === message.conversationId)
    if (!c || message.delivery) return
    const previous = unreadAnchors[c.id]
    unreadAnchors[c.id] = message.id
    // An automatic read already scheduled, or arriving before the boundary does, must not
    // undo this at once.
    clearTimeout(readTimer)
    unreadPending.add(c.id)
    try {
      const saved = await apiClient().updateConversation(c.id, { unreadFrom: message.id })
      inboxChanged(queryCache)
      // The live update usually arrives first and may already count newer messages.
      if (
        data.value?.demo &&
        data.value.conversations.find((x) => x.id === c.id)?.unreadFrom !== message.id
      )
        putConversation(saved)
    } catch (error) {
      if (previous) unreadAnchors[c.id] = previous
      else delete unreadAnchors[c.id]
      notifyError(error)
    } finally {
      unreadPending.delete(c.id)
    }
  }
  function activate(id: string, remember = true) {
    if (id.startsWith('server:')) {
      const target = id.slice('server:'.length)
      if (!data.value?.networks.some((n) => n.id === target)) return
      selectionRevision.value++
      activeId.value = id
      networkId.value = target
      writeLocal(`${accountPrefix}.active`, id, 'session')
      return
    }
    const c = data.value?.conversations.find((x) => x.id === id)
    if (!c) return
    selectionRevision.value++
    if (c.unreadCount) unreadMarkers[id] = c.unreadCount
    else delete unreadMarkers[id]
    if (c.unreadFrom) unreadAnchors[id] = c.unreadFrom
    else delete unreadAnchors[id]
    // Re-selecting the open conversation (a reload restoring it) is not reading it on purpose.
    const switching = activeId.value !== id
    activeId.value = id
    networkId.value = c.networkId
    if (remember) {
      writeLocal(`${accountPrefix}.active`, id, 'session')
      writeLocal(`${accountPrefix}.last-conversation`, id)
    }
    if (readingActiveConversation(id)) void markRead(id, !switching)
  }
  function selectNetwork(id: string) {
    if (!data.value?.networks.some((n) => n.id === id)) return
    const c =
      data.value?.conversations.find((x) => x.networkId === id && x.kind === 'channel') ??
      data.value?.conversations.find((x) => x.networkId === id)
    activate(c?.id ?? serverViewId(id))
  }
  function step(direction: 1 | -1) {
    const list = data.value?.conversations.filter((c) => c.networkId === networkId.value) ?? []
    const i = list.findIndex((c) => c.id === activeId.value)
    const next = list[(i + direction + list.length) % list.length]
    if (next) activate(next.id)
  }

  const loadOlder = (id: string) => pagination.load(id, 'older')
  const loadNewer = (id: string) => pagination.load(id, 'newer')

  /**
   * Replaces a conversation's timeline with a window of history around one message, however
   * far back it is. Throws when the server can't read it (a 404 means it is gone).
   */
  async function openAround(id: string, messageId: string) {
    cancelHistoryReads()
    const controller = new AbortController()
    aroundRead = controller
    const key = messagesQuery(id).key
    try {
      const page = await readMessages(id, { around: messageId, limit: 60 }, controller.signal)
      // A newest-page load started by opening the conversation must not replace the window.
      await queryCache.cancelQueries({ key, exact: true })
      if (controller.signal.aborted || aroundRead !== controller) return
      pagination.cancel()
      queryCache.setQueryData(
        key,
        mergeMessagePage(page, queryCache.getQueryData<MessagePage>(key), !!page.hasNewer),
      )
    } catch (error) {
      if (!controller.signal.aborted) throw error
    } finally {
      if (aroundRead === controller) aroundRead = undefined
    }
  }
  /** Reconcile the authoritative latest page, even if a clipped/stale window looked live. */
  async function returnToPresent(id: string) {
    return settleHistoryRefresh(async () => {
      const key = messagesQuery(id).key
      cancelHistoryReads()
      queryCache.cancelQueries({ key, exact: true })
      return queryCache.fetch(queryCache.ensure(messagesQuery(id)))
    }, notifyError)
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
      void apiClient()
        .sendMessage(message.conversationId, payload)
        .then((accepted) => {
          if (accepted.message) putMessageReply(accepted.message)
        })
        .catch((error) => {
          markFailed(clientId)
          notifyError(error)
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
        putMessageReply(command.message)
        return
      }
      if (command.state === 'uncertain') notify(translate('workspaceUi.chat.deliveryUncertain'))
    } catch {
      /* The durable command remains available when connectivity returns. */
    }
    markFailed(message.clientId!)
  }
  const attachmentIds = new Map<string, string[]>()
  /**
   * Sends to the open conversation, or to `options.conversationId` when given. Markdown-style
   * shortcuts become IRC formatting codes, so other IRC clients see bold and italics too.
   */
  function send(
    text: string,
    replyTo?: Message,
    gif?: GifItem,
    options: {
      kind?: 'message' | 'action'
      upload?: Upload
      uploads?: Upload[]
      conversationId?: string
      /** Send the text as written, without turning markdown into IRC formatting. */
      verbatim?: boolean
    } = {},
  ) {
    const targetId = options.conversationId ?? activeId.value
    const target =
      options.conversationId === undefined
        ? active.value
        : data.value?.conversations.find((c) => c.id === targetId)
    const targetNetwork = target && networkOf(target)
    if (!target || !targetNetwork) {
      notify(translate('workspaceUi.chat.chooseConversation'))
      return false
    }
    const unavailable = conversationSendReason(
      target,
      targetNetwork,
      status.value === 'connected',
      translate,
    )
    if (unavailable) {
      notify(unavailable)
      return false
    }
    if (!text.trim()) return false
    const uploads = options.uploads ?? (options.upload ? [options.upload] : [])
    const wireText = toWireText(text, !!(gif || uploads.length || options.verbatim))
    const reason =
      (replyTo && messageActionReason(replyTo, 'replies')) ||
      messageTextReason(wireText, options.kind ?? 'message', target, targetNetwork, translate)
    if (reason) {
      notify(reason)
      return false
    }
    const clientId = crypto.randomUUID()
    const attachments = uploads.map((upload) => ({
      name: upload.name,
      url: upload.url,
      width: upload.width ?? 0,
      height: upload.height ?? 0,
      alt: upload.name,
      contentType: upload.contentType,
      size: upload.size,
    }))
    if (uploads.length)
      attachmentIds.set(
        clientId,
        uploads.map((upload) => upload.id),
      )
    delete unreadMarkers[targetId]
    delete unreadAnchors[targetId]
    // Replying is reading on purpose, so it ends a manual “mark as unread”.
    if (target.unreadFrom) void markRead(targetId)
    // What you send appears at the live end, so a window into older history goes back there.
    // At the live end already, reloading the newest page would only replace the rows on screen.
    const page = queryCache.getQueryData<MessagePage>(messagesQuery(targetId).key)
    if (!page || page.detached || page.hasNewer) void returnToPresent(targetId)
    const message: Message = {
      id: `pending-${clientId}`,
      clientId,
      conversationId: targetId,
      senderId: 'you',
      text: wireText,
      createdAt: new Date().toISOString(),
      kind: options.kind ?? 'message',
      reactions: [],
      delivery: 'pending',
      ...(gif ? { gif } : {}),
      ...(attachments.length === 1
        ? { attachment: attachments[0] }
        : attachments.length > 1
          ? { attachments }
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
      notify(translate('workspaceUi.chat.notConnected'))
      return false
    }
    if (!data.value?.demo) {
      void apiClient()
        .raw(target, { clientId: crypto.randomUUID(), line })
        .catch((error) => notifyError(error))
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
  function onAlert(listener: (alert: NotificationAlert) => void) {
    alertListeners.add(listener)
    return () => alertListeners.delete(listener)
  }

  // ---- Mutations ------------------------------------------------------------------------------

  async function react(message: Message, emoji: string) {
    const reason = messageActionReason(message, 'reactions')
    if (reason) return notify(reason)
    const previous = cachedMessage(message)
    try {
      putMessageReply(
        await apiClient().react(message.conversationId, { messageId: message.id, emoji }),
        previous,
      )
    } catch (error) {
      notifyError(error)
    }
  }

  /** Makes your newest message here sparkle for everyone; the server's event plays it. */
  async function attract(conversationId: string) {
    try {
      await apiClient().attract(conversationId)
    } catch (error) {
      notifyError(error)
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
    const previous = cachedMessage(message)
    try {
      const result = await apiClient().redact(message.conversationId, message.id)
      putMessageReply(result.message, previous)
      return result
    } catch (error) {
      notifyError(error)
      return undefined
    }
  }
  /**
   * Replaces the text of one of your messages for everyone. Resolves true once the server
   * accepted the edit; the canonical row also arrives as message.updated.
   */
  async function edit(message: Message, text: string) {
    const reason = messageActionReason(message, 'edit')
    if (reason) {
      notify(reason)
      return false
    }
    const wireText = markdownToIrc(text.trim())
    if (!wireText) return false
    if (wireText === message.text) return true
    const previous = cachedMessage(message)
    try {
      const result = await apiClient().edit(message.conversationId, message.id, wireText)
      putMessageReply(result.message, previous)
      return true
    } catch (error) {
      notifyError(error)
      return false
    }
  }
  /** Deletes an account-owned upload; returns whether it succeeded. */
  async function deleteUpload(id: string) {
    try {
      await apiClient().deleteUpload(id)
      return true
    } catch (error) {
      notifyError(error)
      return false
    }
  }

  const favorite = useMutation({
    mutation: (c: Conversation) => apiClient().updateConversation(c.id, { favorite: !c.favorite }),
    onMutate: (c) => {
      if (data.value?.demo) putConversation({ ...c, favorite: !c.favorite })
    },
    onSuccess: (c) => {
      if (data.value?.demo) putConversation(c)
    },
    onError: (error, c) => {
      if (data.value?.demo) putConversation(c)
      notifyError(error)
    },
  })

  /**
   * Channel-wide notification settings for everyone in the
   * channel. Only operators may change them; the backend checks and tells every member.
   */
  const channelSettings = useMutation({
    mutation: ({ c, changes }: { c: Conversation; changes: ChannelSettingsChange }) =>
      apiClient().updateChannelSettings(c.id, changes),
    onMutate: ({ c, changes }) => {
      if (!data.value?.demo) return
      const current = c.channelSettings ?? { notifications: 'default', groupMentions: 'ops' }
      putConversation({
        ...c,
        channelSettings: {
          ...current,
          notifications: changes.notifications ?? current.notifications,
          groupMentions: changes.groupMentions ?? current.groupMentions,
        },
      })
    },
    onSuccess: (c) => {
      if (data.value?.demo) putConversation(c)
    },
    onError: (error, { c }) => {
      if (data.value?.demo) putConversation(c)
      notifyError(error)
    },
  })
  const join = useMutation({
    mutation: (input: string | { name: string; key?: string; networkId?: string }) => {
      const value = typeof input === 'string' ? { name: input } : input
      return apiClient().createConversation({
        networkId: value.networkId ?? networkId.value,
        name: value.name,
        key: value.key,
      })
    },
    onSuccess: (c) => {
      // JOIN acceptance can reach HTTP after the newer roster/joined event.
      // The socket owns live state; only seed a conversation that is still absent.
      if (!data.value?.conversations.some((existing) => existing.id === c.id)) putConversation(c)
      activate(c.id)
    },
  })
  const rejectInvitation = useMutation({
    mutation: (id: string) => apiClient().rejectInvitation(id),
    onSuccess: (_, id) =>
      patchBootstrap((b) => ({
        ...b,
        invitations: (b.invitations ?? []).filter((v) => v.id !== id),
      })),
  })
  const direct = useMutation({
    mutation: (memberId: string) =>
      apiClient().createConversation({ networkId: networkId.value, memberId }),
    onSuccess: (c) => {
      if (!data.value?.conversations.some((existing) => existing.id === c.id)) putConversation(c)
      activate(c.id)
    },
  })
  /** Leaves a channel (PART) or closes a direct conversation. */
  const leave = useMutation({
    mutation: ({ c, reason }: { c: Conversation; reason?: string }) =>
      apiClient().leaveConversation(c.id, reason),
    onSuccess: (_result, { c }) => {
      if (data.value?.demo) leaveRemoved((x) => x.id === c.id)
    },
    onError: (error) => notifyError(error),
  })
  const addNetwork = useMutation({
    mutation: async (input: NetworkInput) => {
      const n = await apiClient().createNetwork(input)
      if (!data.value?.networks.some((existing) => existing.id === n.id)) putNetwork(n)
      // Join the auto-join channels, or a first channel so the network isn't empty.
      const channels = data.value?.demo
        ? input.autoJoin.length
          ? input.autoJoin
          : ['#general']
        : []
      const joined: Conversation[] = []
      for (const entry of channels) {
        const [name, key] = entry.split(' ')
        joined.push(await apiClient().createConversation({ networkId: n.id, name, key }))
      }
      return { network: n, conversations: joined }
    },
    onSuccess: ({ network: n, conversations }) => {
      conversations.forEach(putConversation)
      if (conversations[0]) activate(conversations[0].id)
      else selectNetwork(n.id)
      notify(translate('workspaceUi.chat.networkAdded', { name: n.name }))
    },
  })
  const updateNetwork = useMutation({
    mutation: ({ id, input }: { id: string; input: NetworkInput }) =>
      apiClient().updateNetwork(id, input),
    onSuccess: (n) => {
      if (data.value?.demo) putNetwork(n)
    },
  })
  const removeNetwork = useMutation({
    mutation: (n: Network) => apiClient().deleteNetwork(n.id),
    onSuccess: (_result, n) => {
      if (data.value?.demo) removeNetworkLocally(n.id)
      notify(translate('workspaceUi.chat.networkRemoved', { name: n.name }))
    },
    onError: (error) => notifyError(error),
  })
  const connection = useMutation({
    mutation: (n: Network) =>
      apiClient().updateNetwork(n.id, {
        status:
          n.status === 'connected' || n.status === 'connecting' ? 'disconnected' : 'connected',
      }),
    onSuccess: (n) => {
      if (data.value?.demo) putNetwork(n)
    },
    onError: (error) => notifyError(error),
  })
  const profile = useMutation({
    mutation: (input: ProfileUpdate) => apiClient().profile(input),
    onSuccess: (m) => {
      if (data.value?.demo) putProfile(m)
    },
  })
  const checkpointReady = computed(() => checkpointTrusted.value && !pendingReads.value)

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
  const draftsKey = `${accountPrefix}.drafts`
  const draftTimesKey = `${accountPrefix}.draft-times`
  /** The drafts as this tab last saw them saved: the base for merging other tabs' saves. */
  let savedDrafts = { ...drafts }
  let stampedDrafts = { ...drafts }
  function stampDrafts() {
    stampDraftTimes(stampedDrafts, drafts, draftTimes, Date.now())
    stampedDrafts = { ...drafts }
  }
  /**
   * Takes in what other tabs of this account saved since this tab last looked, so that no
   * write from here (a flush, a discard, an Undo) replaces their newer text with a stale copy.
   */
  function syncDrafts() {
    stampDrafts()
    // Unreadable or missing storage says nothing about other tabs: keep what this tab has.
    const theirs = readLocal(
      draftsKey,
      (v) => (v === undefined ? null : sanitizeDrafts(v, Infinity)),
      'local',
    )
    if (!theirs) return
    const { adopted, seen } = mergeDrafts(savedDrafts, drafts, theirs)
    savedDrafts = seen
    if (!adopted.length) return
    const theirTimes = readLocal(draftTimesKey, (v) => sanitizeDraftTimes(v, theirs), 'local')
    for (const id of adopted) {
      const text = theirs[id]
      if (text) {
        drafts[id] = text
        stampedDrafts[id] = text
      } else {
        delete drafts[id]
        delete stampedDrafts[id]
      }
      if (text && theirTimes[id]) draftTimes[id] = theirTimes[id]
      else delete draftTimes[id]
    }
  }
  /** Writes drafts now instead of after the debounce, e.g. right before a reload. */
  function flushDrafts() {
    clearTimeout(draftTimer)
    syncDrafts()
    draftsSaved.value = writeLocal(draftsKey, drafts, 'local')
    if (draftsSaved.value) savedDrafts = { ...drafts }
    writeLocal(draftTimesKey, draftTimes, 'local')
  }
  watch(drafts, () => {
    stampDrafts()
    clearTimeout(draftTimer)
    draftTimer = setTimeout(flushDrafts, 200)
  })
  // Another tab saved: show its drafts here too (the overview stays current).
  const onStorage = (event: StorageEvent) => {
    if (event.key === draftsKey && event.storageArea === localStorage) syncDrafts()
  }
  /**
   * Discards a draft from the overview, unless it changed since the overview showed it, here or
   * in another tab. Saved at once so other tabs see it.
   */
  function discardDraft(id: string, seen: string) {
    syncDrafts()
    if (!discardDraftText(drafts, id, seen)) return false
    flushDrafts()
    return true
  }
  /** Undoes a discard with its original edit time, never replacing a newer draft. */
  function restoreDraft(id: string, text: string, editedAt?: number) {
    syncDrafts()
    const result = restoreDraftText(drafts, id, text)
    if (result !== 'restored') return result
    stampedDrafts[id] = drafts[id]!
    if (editedAt) draftTimes[id] = editedAt
    flushDrafts()
    return result
  }
  const onOnline = () => recovery.resume(true)
  const onOffline = () => recovery.offline()
  const onVisible = () => {
    if (disposed) return
    recovery.visibility(document.visibilityState !== 'visible')
    if (status.value === 'connected' && readingActiveConversation(activeId.value))
      void markRead(activeId.value, true)
  }
  const onFocus = () => { recovery.resume(); onVisible() }
  const onPageShow = (event: PageTransitionEvent) => {
    if (event.persisted) { recovery.visibility(false); recovery.resume(true) }
  }
  const onPageHide = () => { flushDrafts(); recovery.visibility(true) }
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
  window.addEventListener('focus', onFocus)
  window.addEventListener('pageshow', onPageShow)
  // Like the layout and library stores: keep keystrokes typed just before any reload or close.
  window.addEventListener('pagehide', onPageHide)
  window.addEventListener('storage', onStorage)
  document.addEventListener('visibilitychange', onVisible)
  function dispose() {
    if (disposed) return
    disposed = true
    recovery.dispose()
    cancelRecoveryHistory()
    queryCache.cancelQueries({ key: bootstrapQuery.key, exact: true })
    cancelHistoryReads()
    clearTimeout(memberFlush)
    pendingMembers.clear()
    clearTimeout(noticeTimer)
    clearTimeout(readTimer)
    clearTimeout(draftTimer)
    pendingTimers.forEach(clearTimeout)
    typingTimers.forEach(clearTimeout)
    clearTimeout(typingIdle)
    window.removeEventListener('online', onOnline)
    window.removeEventListener('offline', onOffline)
    window.removeEventListener('focus', onFocus)
    window.removeEventListener('pageshow', onPageShow)
    window.removeEventListener('pagehide', onPageHide)
    window.removeEventListener('storage', onStorage)
    appUpdate.detach()
    document.removeEventListener('visibilitychange', onVisible)
  }
  onScopeDispose(dispose)
  if (import.meta.hot) import.meta.hot.dispose(dispose)

  return {
    bootstrap,
    data,
    self,
    liveSince,
    reconnects,
    lastEventAt,
    activeId,
    lastConversation,
    selectionRevision,
    finishStartup,
    networkId,
    status,
    recoveryFailed,
    historyProblem,
    historyRefreshing,
    refreshHistory,
    notice,
    noticeError,
    holdNotice,
    drafts,
    draftTimes,
    draftsSaved,
    flushDrafts,
    discardDraft,
    restoreDraft,
    outbox,
    loadingOlder,
    loadingNewer,
    unreadMarkers,
    unreadAnchors,
    pendingInsert,
    serverView,
    active,
    network,
    canSend,
    sendReason,
    canSendTo,
    // Getters returning the function, not plain functions: Pinia wraps returned functions as
    // actions (argument copies, listener bookkeeping on every call), and these lookups run
    // thousands of times per update in big channels. Callers still write chat.member(id).
    activeMemberIds,
    member: computed(() => member),
    memberByNick: computed(() => memberByNick),
    recipientFor,
    directPartner: computed(() => directPartner),
    networkOf: computed(() => networkOf),
    messageActionReason: computed(() => messageActionReason),
    messageActionReasonKey: computed(() => messageActionReasonKey),
    messageActionSupported: computed(() => messageActionSupported),
    notify,
    notifyError,
    activate,
    selectNetwork,
    step,
    markRead,
    markUnread,
    loadOlder,
    loadNewer,
    openAround,
    returnToPresent,
    send,
    raw,
    retry,
    discard,
    onIncoming,
    onAlert,
    routing,
    tabId,
    react,
    redact,
    edit,
    deleteUpload,
    favorite,
    channelSettings,
    typing,
    attention,
    attract,
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
    saveCheckpoint,
    checkpointReady,
    cursor,
    dispose,
  }
})

if (import.meta.hot) import.meta.hot.accept(acceptHMRUpdate(useChatStore, import.meta.hot))
