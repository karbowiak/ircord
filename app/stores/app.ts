import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Server, Channel, DMChannel, Message, ChannelMember } from '~/types'
import { useChatDataSource } from '~/composables/useChatDataSource'
import { useMessageDb } from '~/composables/useMessageDb'
import type { StoredMessage } from '~/composables/useMessageDb'
import { IrcAdapter } from '~/irc/adapter'
import type { IrcIncomingMessageEvent, RawLogEntry, NamesEntry, ChannelBanEntry, WhoisInfo } from '~/irc/adapter'
import { parseIrcMessage, parseNickname } from '~/irc/protocol'
import { WebSocketIrcTransport } from '~/irc/transport'

const LAST_CHANNEL_KEY = 'ircord-last-channel-by-server'
const GROUPS_KEY = 'ircord-custom-groups-by-server'
const COLLAPSED_SECTIONS_KEY = 'ircord-collapsed-sections-by-server'
const SETTINGS_KEY = 'ircord-client-settings'
const CHANNEL_TOPICS_KEY = 'ircord-channel-topics'
const MESSAGE_REACTIONS_KEY = 'ircord-message-reactions'
const IRC_SERVERS_KEY = 'ircord-irc-servers'

type ServerChannelMemory = Record<string, string>
type SidebarSectionKey = 'dms' | 'channels' | 'groups'

type CollapsedSections = Record<string, Record<SidebarSectionKey, boolean>>

type CustomGroup = {
  id: string
  name: string
  collapsed: boolean
  itemIds: string[]
}

type GroupsByServer = Record<string, CustomGroup[]>
type ChannelTopicsById = Record<string, string>

type IrcServerConfig = {
  id: string
  name: string
  abbreviation: string
  webSocketUrl: string
  serverName: string
  nick: string
  username: string
  realname: string
  serverPassword?: string
  servicesAccount: string
  servicesPassword: string
  autoOwnershipTake: boolean
  password?: string
  channels: string[]
}

type IrcMessageMeta = {
  serverId: string
  channelName: string
  msgid: string | null
}

type ChannelSettingsState = {
  topic: string
  modes: string
  bans: ChannelBanEntry[]
  loading: boolean
  error: string | null
}

type ChannelParticipationState = {
  joined: boolean
  reason: 'kick' | 'ban' | 'part' | null
  detail: string | null
  canRejoin: boolean
}

type ChannelServiceActivityEntry = {
  id: string
  timestamp: string
  author: string
  content: string
}

type WhoisModalState = {
  open: boolean
  loading: boolean
  nick: string
  error: string | null
  info: WhoisInfo | null
}

type IrcConnectInput = {
  name: string
  webSocketUrl: string
  serverName: string
  nick: string
  username: string
  realname?: string
  serverPassword?: string
  servicesAccount: string
  servicesPassword: string
  autoOwnershipTake?: boolean
  password?: string
  channels: string[]
}

type ClientSettings = {
  gifAutoplay: boolean
  gifFavorites: GifFavorite[]
  mediaScalePercent: number
  recentReactions: string[]
}

type GifFavorite = {
  id: string
  title: string
  url: string
  playbackUrl: string
  previewUrl: string
  width?: number
  height?: number
  addedAt: string
}

export const useAppStore = defineStore('app', () => {
  const dataSource = useChatDataSource()
  const isIrcMode = dataSource.source === 'irc'
  const usersState = ref(dataSource.users.length ? [...dataSource.users] : [
    {
      id: 'local-user',
      username: dataSource.ircDefaults.nick,
      avatar: dataSource.ircDefaults.nick.slice(0, 1).toUpperCase() || 'U',
      status: 'online' as const,
    },
  ])
  const serversState = ref<Server[]>([...dataSource.servers])
  const dmsState = ref<DMChannel[]>([...dataSource.dms])
  const messagesState = ref<Message[]>([...dataSource.messages])
  const ircServers = ref<IrcServerConfig[]>(loadLocalJson<IrcServerConfig[]>(IRC_SERVERS_KEY, []))
  const ircAdapters = new Map<string, IrcAdapter>()
  const ircMessageMetaByLocalId = ref<Record<string, IrcMessageMeta>>({})
  const ircLocalIdByMsgid = ref<Record<string, string>>({})
  const pendingReplyLinksByParentMsgid = ref<Record<string, string[]>>({})
  const pendingReactionsByParentMsgid = ref<Record<string, string[]>>({})
  const ownershipPromptedByChannel = ref<Record<string, boolean>>({})
  const initializingIrc = ref(false)
  const messageDb = useMessageDb()
  const ircServerStatus = ref<Record<string, {
    rawLog: RawLogEntry[]
    latencyMs: number | null
    nick: string
    userModes: string
    capabilities: string[]
    connectedAt: string | null
  }>>({})
  const ircLatencyIntervals = new Map<string, ReturnType<typeof setInterval>>()

  const currentUser = ref(usersState.value[0])
  const activeServerId = ref<string | null>(null)
  const activeChannelId = ref<string | null>(null)
  const settings = ref<ClientSettings>({
    gifAutoplay: true,
    gifFavorites: [],
    mediaScalePercent: 100,
    recentReactions: ['👍', '😂', '🔥'],
    ...loadLocalJson<ClientSettings>(SETTINGS_KEY, { gifAutoplay: true, gifFavorites: [], mediaScalePercent: 100, recentReactions: ['👍', '😂', '🔥'] }),
  })
  const lastChannelByServer = ref<ServerChannelMemory>(loadLocalJson(LAST_CHANNEL_KEY, {}))
  const customGroupsByServer = ref<GroupsByServer>(loadLocalJson(GROUPS_KEY, {}))
  const collapsedSectionsByServer = ref<CollapsedSections>(loadLocalJson(COLLAPSED_SECTIONS_KEY, {}))
  const channelTopicsById = ref<ChannelTopicsById>(loadLocalJson(CHANNEL_TOPICS_KEY, {}))
  const messageReactionsById = ref<Record<string, string[]>>(loadLocalJson(MESSAGE_REACTIONS_KEY, {}))
  const channelSettingsById = ref<Record<string, ChannelSettingsState>>({})
  const channelParticipationById = ref<Record<string, ChannelParticipationState>>({})
  const channelServiceActivityById = ref<Record<string, ChannelServiceActivityEntry[]>>({})
  const whoisState = ref<WhoisModalState>({
    open: false,
    loading: false,
    nick: '',
    error: null,
    info: null,
  })
  const replyTargetMessageId = ref<string | null>(null)

  const serversList = computed(() => serversState.value)
  const dmsList = computed(() => dmsState.value)
  const channelsList = computed(() => {
    if (!activeServerId.value) return []
    return serversState.value.find(s => s.id === activeServerId.value)?.channels || []
  })
  const customGroupsForActiveServer = computed(() => {
    if (!activeServerId.value) return []
    return customGroupsByServer.value[activeServerId.value] || []
  })
  const collapsedSectionsForActiveServer = computed<Record<SidebarSectionKey, boolean>>(() => {
    if (!activeServerId.value) return defaultCollapsedSections()
    return collapsedSectionsByServer.value[activeServerId.value] || defaultCollapsedSections()
  })
  const gifAutoplay = computed(() => settings.value.gifAutoplay)
  const gifFavorites = computed(() => settings.value.gifFavorites)
  const mediaScalePercent = computed(() => {
    const clamped = Math.max(10, Math.min(100, Number(settings.value.mediaScalePercent) || 100))
    return clamped
  })
  const recentReactions = computed(() => settings.value.recentReactions.slice(0, 3))
  const messagesList = computed(() => {
    if (!activeChannelId.value) return []
    return messagesState.value.filter((message) => {
      if (message.channelId !== activeChannelId.value) return false
      return !isServiceActivityAuthor(message.author.username)
    })
  })

  const activeServer = computed(() =>
    serversState.value.find(s => s.id === activeServerId.value) || null
  )
  const activeServerStatus = computed(() => {
    if (!activeServerId.value) return null
    return ircServerStatus.value[activeServerId.value] || null
  })
  const activeChannel = computed((): Channel | DMChannel | null => {
    if (!activeChannelId.value) return null
    const channel = channelsList.value.find(c => c.id === activeChannelId.value)
    if (channel) return channel
    return dmsState.value.find(d => d.id === activeChannelId.value) || null
  })
  const activeChannelSettings = computed(() => {
    if (!activeChannelId.value) return null
    return channelSettingsById.value[activeChannelId.value] || null
  })
  const activeChannelParticipation = computed(() => {
    if (!activeChannelId.value) return null
    return channelParticipationById.value[activeChannelId.value] || null
  })
  const activeChannelServiceActivity = computed(() => {
    if (!activeChannelId.value) return []
    return channelServiceActivityById.value[activeChannelId.value] || []
  })

  const channelMembersList = computed((): ChannelMember[] => {
    if (!activeChannelId.value) return []
    const channel = channelsList.value.find(c => c.id === activeChannelId.value)
    if (!channel) return []
    const modeOrder = { op: 0, voice: 1, regular: 2 }
    return [...channel.members].sort((a, b) => {
      const modeDiff = modeOrder[a.mode] - modeOrder[b.mode]
      if (modeDiff !== 0) return modeDiff
      return a.user.username.localeCompare(b.user.username)
    })
  })

  function isDmChannel(channelId: string): boolean {
    return dmsState.value.some(dm => dm.id === channelId)
  }

  function isChannelInServer(channelId: string, serverId: string): boolean {
    return serversState.value.some(server =>
      server.id === serverId && server.channels.some(channel => channel.id === channelId)
    )
  }

  function setActiveServer(serverId: string) {
    if (!serversState.value.some(server => server.id === serverId)) return

    activeServerId.value = serverId

    const remembered = lastChannelByServer.value[serverId]
    if (remembered && isChannelInServer(remembered, serverId)) {
      activeChannelId.value = remembered
      return
    }

    activeChannelId.value = null
  }

  function setActiveChannel(channelId: string) {
    const inServer = activeServerId.value
      ? isChannelInServer(channelId, activeServerId.value)
      : false
    const isDm = isDmChannel(channelId)

    if (!inServer && !isDm) return

    activeChannelId.value = channelId

    // Load persisted messages for this channel from IndexedDB
    loadChannelMessagesFromDb(channelId)

    if (activeServerId.value && inServer) {
      lastChannelByServer.value = {
        ...lastChannelByServer.value,
        [activeServerId.value]: channelId,
      }
      saveLocalJson(LAST_CHANNEL_KEY, lastChannelByServer.value)
    }
  }

  async function loadChannelMessagesFromDb(channelId: string) {
    try {
      const stored = await messageDb.getChannelMessages(channelId)
      if (!stored.length) return

      // Merge: keep any in-memory messages not in DB, add DB messages not in memory
      const existingIds = new Set(messagesState.value.map(m => m.id))
      const newFromDb = stored.filter(m => !existingIds.has(m.id))
      if (newFromDb.length) {
        const regularMessages = [] as typeof newFromDb
        for (const message of newFromDb) {
          if (isServiceActivityAuthor(message.author.username)) {
            addChannelServiceActivity(channelId, message.author.username, message.content, message.timestamp)
            continue
          }
          regularMessages.push(message)
        }

        if (regularMessages.length) {
          messagesState.value = [...messagesState.value, ...regularMessages]
        }

        // Rebuild msgid→localId mapping for persisted messages
        for (const m of regularMessages) {
          const sm = m as StoredMessage
          if (sm.ircMsgid) {
            ircLocalIdByMsgid.value[sm.ircMsgid] = sm.id
          }
        }
      }
    } catch (err) {
      console.warn('[ircord] Failed to load messages from IndexedDB', err)
    }
  }

  function getDefaultChannelTopic(channelId: string): string {
    for (const server of serversState.value) {
      const channel = server.channels.find(entry => entry.id === channelId)
      if (channel) return channel.topic || ''
    }
    return ''
  }

  function getChannelTopic(channelId: string): string {
    if (channelTopicsById.value[channelId] !== undefined) {
      return channelTopicsById.value[channelId]
    }
    return getDefaultChannelTopic(channelId)
  }

  function setLocalChannelTopic(channelId: string, topic: string) {
    if (!serversState.value.some(server => server.channels.some(channel => channel.id === channelId))) return

    const normalized = topic.trim()
    const defaultTopic = getDefaultChannelTopic(channelId)

    if (normalized === defaultTopic) {
      const { [channelId]: _, ...rest } = channelTopicsById.value
      channelTopicsById.value = rest
      saveLocalJson(CHANNEL_TOPICS_KEY, channelTopicsById.value)
      return
    }

    channelTopicsById.value = {
      ...channelTopicsById.value,
      [channelId]: normalized,
    }

    saveLocalJson(CHANNEL_TOPICS_KEY, channelTopicsById.value)

    for (const server of serversState.value) {
      const channel = server.channels.find(entry => entry.id === channelId)
      if (!channel) continue
      channel.topic = normalized
      serversState.value = [...serversState.value]
      return
    }
  }

  function getChannelParticipation(channelId: string): ChannelParticipationState {
    return channelParticipationById.value[channelId] || {
      joined: true,
      reason: null,
      detail: null,
      canRejoin: true,
    }
  }

  function setChannelParticipation(channelId: string, next: Partial<ChannelParticipationState>) {
    const current = getChannelParticipation(channelId)
    channelParticipationById.value = {
      ...channelParticipationById.value,
      [channelId]: {
        ...current,
        ...next,
      },
    }
  }

  function addChannelSystemNotice(channelId: string, content: string) {
    const serverUser = ensureIrcUser('server')
    messagesState.value.push({
      id: `irc-system-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      channelId,
      author: serverUser,
      content,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    })
  }

  function isServiceActivityAuthor(username: string): boolean {
    const normalized = username.trim().toLowerCase()
    return normalized === 'histserv' || normalized === 'chanserv'
  }

  function addChannelServiceActivity(channelId: string, author: string, content: string, timestamp: string) {
    const line = content.trim()
    if (!line) return

    const current = channelServiceActivityById.value[channelId] || []
    const next = [
      ...current,
      {
        id: `svc-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        timestamp,
        author,
        content: line,
      },
    ]

    channelServiceActivityById.value = {
      ...channelServiceActivityById.value,
      [channelId]: next.slice(-80),
    }
  }

  function getCurrentUserChannelMode(channelId: string): 'op' | 'voice' | 'regular' {
    const channel = serversState.value
      .flatMap(server => server.channels)
      .find(entry => entry.id === channelId)
    if (!channel) return 'regular'

    const nick = activeServerStatus.value?.nick || currentUser.value.username
    const member = channel.members.find(entry => entry.user.username === nick)
    return member?.mode || 'regular'
  }

  function canEditChannelTopic(channelId: string): boolean {
    if (!isIrcMode) return true
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    const channel = serversState.value
      .find(server => server.id === mapping.serverId)
      ?.channels.find(entry => entry.id === channelId)
    if (!channel) return false

    const userModes = ircServerStatus.value[mapping.serverId]?.userModes || ''
    if (userModes.includes('o')) return true

    const requiresOps = (channel.modes || '').includes('t')
    if (!requiresOps) return true
    return getCurrentUserChannelMode(channelId) === 'op'
  }

  function canManageChannelBans(channelId: string): boolean {
    if (!isIrcMode) return true
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    const userModes = ircServerStatus.value[mapping.serverId]?.userModes || ''
    if (userModes.includes('o')) return true
    return getCurrentUserChannelMode(channelId) === 'op'
  }

  async function setChannelTopic(channelId: string, topic: string): Promise<boolean> {
    if (!isIrcMode) {
      setLocalChannelTopic(channelId, topic)
      return true
    }

    if (!canEditChannelTopic(channelId)) return false

    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return false

    const changed = await adapter.setTopic(mapping.channelName, topic)
    if (!changed) return false

    setLocalChannelTopic(channelId, topic)
    return true
  }

  function canModerateChannelMember(channelId: string, targetUsername: string, targetMode: ChannelMode): boolean {
    if (!isIrcMode) return false
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    const actorNick = ircServerStatus.value[mapping.serverId]?.nick || currentUser.value.username
    if (targetUsername === actorNick) return false

    const actorMode = getCurrentUserChannelMode(channelId)
    const actorRank = channelModeRank(actorMode)
    const userModes = ircServerStatus.value[mapping.serverId]?.userModes || ''
    const isOper = userModes.includes('o')
    if (actorMode !== 'op' && !isOper) return false

    if (isOper) {
      return targetMode !== 'op'
    }

    return actorRank > channelModeRank(targetMode)
  }

  async function kickChannelMember(channelId: string, targetUsername: string): Promise<boolean> {
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return false

    const server = serversState.value.find(entry => entry.id === mapping.serverId)
    const channel = server?.channels.find(entry => entry.id === channelId)
    const member = channel?.members.find(entry => entry.user.username === targetUsername)
    const targetMode = member?.mode || 'regular'
    if (!canModerateChannelMember(channelId, targetUsername, targetMode)) return false

    return adapter.kickUser(mapping.channelName, targetUsername)
  }

  async function banChannelMember(channelId: string, targetUsername: string): Promise<boolean> {
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return false

    const server = serversState.value.find(entry => entry.id === mapping.serverId)
    const channel = server?.channels.find(entry => entry.id === channelId)
    const member = channel?.members.find(entry => entry.user.username === targetUsername)
    const targetMode = member?.mode || 'regular'
    if (!canModerateChannelMember(channelId, targetUsername, targetMode)) return false

    const banned = await adapter.banUser(mapping.channelName, targetUsername)
    if (!banned) return false
    await adapter.kickUser(mapping.channelName, targetUsername, 'Banned from channel')
    return true
  }

  async function addChannelBan(channelId: string, mask: string): Promise<boolean> {
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false

    if (!canManageChannelBans(channelId)) return false

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return false

    const applied = await adapter.setBanMask(mapping.channelName, mask)
    if (!applied) return false

    await refreshChannelSettings(channelId)
    return true
  }

  async function setChannelModes(channelId: string, modeSpec: string): Promise<boolean> {
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false
    if (!canManageChannelBans(channelId)) return false

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return false

    const trimmed = modeSpec.trim()
    if (!trimmed) return false

    if (/\s/.test(trimmed)) {
      const changed = await adapter.setChannelModes(mapping.channelName, trimmed)
      if (!changed) return false
      await refreshChannelSettings(channelId)
      return true
    }

    const channel = serversState.value
      .find(entry => entry.id === mapping.serverId)
      ?.channels.find(entry => entry.id === channelId)

    const currentModes = extractSimpleModeLetters(channel?.modes || '')
    const desiredModes = extractSimpleModeLetters(trimmed)
    const toAdd = [...desiredModes].filter(mode => !currentModes.has(mode)).join('')
    const toRemove = [...currentModes].filter(mode => !desiredModes.has(mode)).join('')

    if (!toAdd && !toRemove) return true

    if (toAdd) {
      const added = await adapter.setChannelModes(mapping.channelName, `+${toAdd}`)
      if (!added) return false
    }

    if (toRemove) {
      const removed = await adapter.setChannelModes(mapping.channelName, `-${toRemove}`)
      if (!removed) return false
    }

    await refreshChannelSettings(channelId)
    return true
  }

  async function renameChannel(channelId: string, nextNameInput: string): Promise<boolean> {
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return false
    if (!canManageChannelBans(channelId)) return false

    const nextNameRaw = nextNameInput.trim()
    if (!nextNameRaw) return false
    const nextName = nextNameRaw.startsWith('#') ? nextNameRaw : `#${nextNameRaw}`

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return false

    const renamed = await adapter.renameChannel(mapping.channelName, nextName)
    if (!renamed) return false

    applyChannelRename(mapping.serverId, mapping.channelName, nextName)
    return true
  }

  async function refreshChannelSettings(channelId: string): Promise<void> {
    const mapping = parseLocalChannelId(channelId)
    if (!mapping) return

    const adapter = ircAdapters.get(mapping.serverId)
    if (!adapter) return

    const current = channelSettingsById.value[channelId] || {
      topic: getChannelTopic(channelId),
      modes: '',
      bans: [],
      loading: false,
      error: null,
    }

    channelSettingsById.value = {
      ...channelSettingsById.value,
      [channelId]: {
        ...current,
        loading: true,
        error: null,
      },
    }

    try {
      const [topic, modes, bans] = await Promise.all([
        adapter.requestChannelTopic(mapping.channelName),
        adapter.requestChannelModes(mapping.channelName),
        adapter.requestBanList(mapping.channelName),
      ])

      setLocalChannelTopic(channelId, topic)
      applyChannelModes(channelId, modes)

      channelSettingsById.value = {
        ...channelSettingsById.value,
        [channelId]: {
          topic,
          modes,
          bans,
          loading: false,
          error: null,
        },
      }
    } catch {
      channelSettingsById.value = {
        ...channelSettingsById.value,
        [channelId]: {
          ...current,
          loading: false,
          error: 'Failed to load channel settings',
        },
      }
    }
  }

  function selectServerStatus() {
    activeChannelId.value = null
  }

  function createCustomGroup(name: string) {
    if (!activeServerId.value) return

    const trimmed = name.trim()
    if (!trimmed) return

    updateCustomGroupsForActiveServer((current) => [
      ...current,
      {
        id: `group-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        name: trimmed,
        collapsed: false,
        itemIds: [],
      },
    ])
  }

  function toggleCustomGroupCollapsed(groupId: string) {
    updateCustomGroupsForActiveServer(current => current.map((group) => {
      if (group.id !== groupId) return group
      return { ...group, collapsed: !group.collapsed }
    }))
  }

  function deleteCustomGroup(groupId: string) {
    updateCustomGroupsForActiveServer(current => current.filter(group => group.id !== groupId))
  }

  function assignItemToGroup(itemId: string, groupId: string | null) {
    updateCustomGroupsForActiveServer((current) => {
      const withoutItem = current.map(group => ({
        ...group,
        itemIds: group.itemIds.filter(id => id !== itemId),
      }))

      if (!groupId) {
        return withoutItem
      }

      return withoutItem.map(group => {
        if (group.id !== groupId) return group
        if (group.itemIds.includes(itemId)) return group
        return { ...group, itemIds: [...group.itemIds, itemId] }
      })
    })
  }

  function moveGroupItemToIndex(groupId: string, itemId: string, targetIndex: number) {
    updateCustomGroupsForActiveServer((current) => current.map((group) => {
      if (group.id !== groupId) return group

      const from = group.itemIds.indexOf(itemId)
      if (from === -1) return group

      const bounded = Math.max(0, Math.min(targetIndex, group.itemIds.length - 1))
      if (from === bounded) return group

      const reordered = [...group.itemIds]
      const [item] = reordered.splice(from, 1)
      reordered.splice(bounded, 0, item)

      return { ...group, itemIds: reordered }
    }))
  }

  function moveCustomGroupToIndex(groupId: string, targetIndex: number) {
    updateCustomGroupsForActiveServer((current) => {
      const reordered = [...current]
      const from = reordered.findIndex(group => group.id === groupId)
      if (from === -1) return current

      const bounded = Math.max(0, Math.min(targetIndex, reordered.length - 1))
      if (from === bounded) return current

      const [group] = reordered.splice(from, 1)
      reordered.splice(bounded, 0, group)
      return reordered
    })
  }

  function getItemGroupId(itemId: string): string | null {
    const group = customGroupsForActiveServer.value.find(entry => entry.itemIds.includes(itemId))
    return group?.id || null
  }

  function updateCustomGroupsForActiveServer(updater: (groups: CustomGroup[]) => CustomGroup[]) {
    if (!activeServerId.value) return

    const serverId = activeServerId.value
    const current = customGroupsByServer.value[serverId] || []
    const next = updater(current)

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [serverId]: next,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function toggleSectionCollapsed(section: SidebarSectionKey) {
    if (!activeServerId.value) return

    const current = collapsedSectionsByServer.value[activeServerId.value] || defaultCollapsedSections()
    const next = { ...current, [section]: !current[section] }

    collapsedSectionsByServer.value = {
      ...collapsedSectionsByServer.value,
      [activeServerId.value]: next,
    }

    saveLocalJson(COLLAPSED_SECTIONS_KEY, collapsedSectionsByServer.value)
  }

  function setGifAutoplay(enabled: boolean) {
    settings.value = {
      ...settings.value,
      gifAutoplay: enabled,
    }

    saveLocalJson(SETTINGS_KEY, settings.value)
  }

  function setMediaScalePercent(percent: number) {
    const clamped = Math.max(10, Math.min(100, Math.round(percent)))

    settings.value = {
      ...settings.value,
      mediaScalePercent: clamped,
    }

    saveLocalJson(SETTINGS_KEY, settings.value)
  }

  function addRecentReaction(emoji: string) {
    const value = emoji.trim()
    if (!value) return

    const next = [value, ...settings.value.recentReactions.filter(item => item !== value)].slice(0, 12)
    settings.value = {
      ...settings.value,
      recentReactions: next,
    }

    saveLocalJson(SETTINGS_KEY, settings.value)
  }

  function toggleGifFavorite(gif: Omit<GifFavorite, 'addedAt'>) {
    const key = gif.playbackUrl || gif.url
    const existing = settings.value.gifFavorites
    const index = existing.findIndex(entry => (entry.playbackUrl || entry.url) === key)

    if (index >= 0) {
      const next = existing.filter((_, i) => i !== index)
      settings.value = {
        ...settings.value,
        gifFavorites: next,
      }
      saveLocalJson(SETTINGS_KEY, settings.value)
      return
    }

    const next = [
      {
        ...gif,
        addedAt: new Date().toISOString(),
      },
      ...existing,
    ].slice(0, 200)

    settings.value = {
      ...settings.value,
      gifFavorites: next,
    }
    saveLocalJson(SETTINGS_KEY, settings.value)
  }

  function getMessageById(messageId: string): Message | null {
    return messagesState.value.find(message => message.id === messageId) || null
  }

  function getMessageReactions(messageId: string): string[] {
    return messageReactionsById.value[messageId] || []
  }

  function toggleMessageReaction(messageId: string, emoji: string) {
    const message = getMessageById(messageId)
    if (!message) return

    const reactions = messageReactionsById.value[messageId] || []
    const exists = reactions.includes(emoji)
    const next = exists ? reactions.filter(item => item !== emoji) : [...reactions, emoji]

    messageReactionsById.value = {
      ...messageReactionsById.value,
      [messageId]: next,
    }

    saveLocalJson(MESSAGE_REACTIONS_KEY, messageReactionsById.value)
    addRecentReaction(emoji)

    if (!isIrcMode) return

    const messageMeta = ircMessageMetaByLocalId.value[messageId]
    if (!messageMeta || !messageMeta.msgid) return

    const adapter = ircAdapters.get(messageMeta.serverId)
    if (!adapter) return

    void adapter.addReaction(messageMeta.channelName, messageMeta.msgid, emoji)
  }

  function setReplyTarget(messageId: string | null) {
    if (!messageId) {
      replyTargetMessageId.value = null
      return
    }

    if (!getMessageById(messageId)) return
    replyTargetMessageId.value = messageId
  }

  const replyTargetMessage = computed(() => {
    if (!replyTargetMessageId.value) return null
    return getMessageById(replyTargetMessageId.value)
  })

  function editMockMessage(messageId: string, content: string) {
    const text = content.trim()
    if (!text) return false

    const index = messagesState.value.findIndex(message => message.id === messageId)
    if (index === -1) return false

    const original = messagesState.value[index]
    if (original.author.id !== currentUser.value.id) return false

    if (isIrcMode) {
      const meta = ircMessageMetaByLocalId.value[messageId]
      if (!meta) return false

      const adapter = ircAdapters.get(meta.serverId)
      if (!adapter) return false

      void adapter.editMessage(meta.channelName, meta.msgid || messageId, text)
    }

    messagesState.value[index] = {
      ...original,
      content: text,
      timestamp: `${original.timestamp} (edited)`,
    }

    return true
  }

  function applyNavigationState(serverId: string | null, channelId: string | null) {
    const fallbackServerId = serversState.value[0]?.id || null
    const nextServerId = serverId && serversState.value.some(server => server.id === serverId)
      ? serverId
      : fallbackServerId

    activeServerId.value = nextServerId

    if (!channelId) {
      if (nextServerId) {
        const remembered = lastChannelByServer.value[nextServerId]
        activeChannelId.value = remembered && isChannelInServer(remembered, nextServerId)
          ? remembered
          : null
      } else {
        activeChannelId.value = null
      }
      return
    }

    const inActiveServer = nextServerId ? isChannelInServer(channelId, nextServerId) : false
    const dmExists = isDmChannel(channelId)

    activeChannelId.value = (inActiveServer || dmExists) ? channelId : null

    if (nextServerId && inActiveServer) {
      lastChannelByServer.value = {
        ...lastChannelByServer.value,
        [nextServerId]: channelId,
      }
      saveLocalJson(LAST_CHANNEL_KEY, lastChannelByServer.value)
    }
  }

  function resetSelection() {
    activeServerId.value = serversState.value[0]?.id || null
    activeChannelId.value = null
  }

  function sendMockMessage(content: string): boolean {
    const text = content.trim()
    if (!text || !activeChannelId.value) return false

    if (isIrcMode) {
      const mapping = parseLocalChannelId(activeChannelId.value)
      if (!mapping) return false

      const participation = getChannelParticipation(activeChannelId.value)
      if (!participation.joined) return false

      const adapter = ircAdapters.get(mapping.serverId)
      if (!adapter) return false

      const parentMeta = replyTargetMessage.value ? ircMessageMetaByLocalId.value[replyTargetMessage.value.id] : null

      if (parentMeta?.msgid) {
        void adapter.replyToMessage(parentMeta.channelName, parentMeta.msgid, text)
      } else {
        void adapter.sendMessage(mapping.channelName, text)
      }

      replyTargetMessageId.value = null
      return true
    }

    const replyTo = replyTargetMessage.value
      ? {
        messageId: replyTargetMessage.value.id,
        authorUsername: replyTargetMessage.value.author.username,
        content: replyTargetMessage.value.content,
      }
      : undefined

    messagesState.value.push({
      id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      channelId: activeChannelId.value,
      author: currentUser.value,
      content: text,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      replyTo,
    })

    replyTargetMessageId.value = null

    return true
  }

  async function connectIrcServer(input: IrcConnectInput): Promise<boolean> {
    if (!isIrcMode) return false

    const config = normalizeIrcConfig(input)
    const endpointKey = serverEndpointKey(config.webSocketUrl, config.serverName)
    const existing = ircServers.value.find(entry =>
      entry.id === config.id || serverEndpointKey(entry.webSocketUrl, entry.serverName) === endpointKey,
    )
    const merged = existing ? {
      ...existing,
      ...config,
      id: existing.id,
      channels: uniqueChannelNames([...existing.channels, ...config.channels]),
    } : config

    const existingAdapter = ircAdapters.get(merged.id)
    if (existingAdapter) {
      existingAdapter.disconnect()
      ircAdapters.delete(merged.id)
    }

    ircServers.value = [
      ...ircServers.value.filter(entry => entry.id !== merged.id),
      merged,
    ]
    saveLocalJson(IRC_SERVERS_KEY, ircServers.value)

    // Set active server BEFORE connecting so channels appear in sidebar as they're joined
    if (!activeServerId.value) {
      ensureIrcServer(merged)
      setActiveServer(merged.id)
    }

    const connected = await connectAndSyncIrcServer(merged)
    if (!connected) return false

    // Select the first channel if one exists
    if (activeServerId.value === merged.id && !activeChannelId.value) {
      const firstChannel = serversState.value.find(server => server.id === merged.id)?.channels[0]
      if (firstChannel) setActiveChannel(firstChannel.id)
    }

    return true
  }

  async function initializeIrcConnections() {
    if (!isIrcMode || initializingIrc.value) return

    initializingIrc.value = true
    try {
      if (!ircServers.value.length) {
        return
      }

      // Set active server BEFORE connecting so channels appear immediately
      if (!activeServerId.value && ircServers.value.length) {
        const firstConfig = ircServers.value[0]
        ensureIrcServer(firstConfig)
        setActiveServer(firstConfig.id)
      }

      for (const server of ircServers.value) {
        await connectAndSyncIrcServer(server)
      }
    } finally {
      initializingIrc.value = false
    }
  }

  async function joinIrcChannel(channelNameInput: string): Promise<boolean> {
    if (!isIrcMode || !activeServerId.value) return false

    const config = ircServers.value.find(entry => entry.id === activeServerId.value)
    const adapter = ircAdapters.get(activeServerId.value)
    if (!config || !adapter) return false

    const channelName = normalizeChannelName(channelNameInput)
    try {
      await adapter.joinChannel(channelName)
      const localChannelId = ensureIrcChannel(config.id, channelName)
      if (!config.channels.includes(channelName)) {
        config.channels = [...config.channels, channelName]
        ircServers.value = [...ircServers.value]
        saveLocalJson(IRC_SERVERS_KEY, ircServers.value)
      }

      await syncIrcChannelMembersFromNames(config.id, channelName)
      setChannelParticipation(localChannelId, {
        joined: true,
        reason: null,
        detail: null,
        canRejoin: true,
      })

      setActiveChannel(localChannelId)
      return true
    } catch {
      return false
    }
  }

  async function rejoinActiveChannel(): Promise<boolean> {
    if (!activeChannelId.value) return false
    const mapping = parseLocalChannelId(activeChannelId.value)
    if (!mapping) return false
    if (!activeServerId.value || activeServerId.value !== mapping.serverId) {
      setActiveServer(mapping.serverId)
    }
    return joinIrcChannel(mapping.channelName)
  }

  async function openWhoisModal(nick: string): Promise<void> {
    const targetNick = nick.trim()
    if (!targetNick) return

    whoisState.value = {
      open: true,
      loading: true,
      nick: targetNick,
      error: null,
      info: null,
    }

    if (!isIrcMode) {
      whoisState.value = {
        ...whoisState.value,
        loading: false,
        error: 'WHOIS is only available in IRC mode.',
      }
      return
    }

    let serverId = activeServerId.value
    if (activeChannelId.value) {
      const mapping = parseLocalChannelId(activeChannelId.value)
      if (mapping) serverId = mapping.serverId
    }

    if (!serverId) {
      whoisState.value = {
        ...whoisState.value,
        loading: false,
        error: 'No active IRC server.',
      }
      return
    }

    const adapter = ircAdapters.get(serverId)
    if (!adapter) {
      whoisState.value = {
        ...whoisState.value,
        loading: false,
        error: 'IRC adapter is not connected.',
      }
      return
    }

    const info = await adapter.requestWhois(targetNick)
    if (!info) {
      whoisState.value = {
        ...whoisState.value,
        loading: false,
        error: 'WHOIS lookup failed or user not found.',
      }
      return
    }

    whoisState.value = {
      open: true,
      loading: false,
      nick: targetNick,
      error: null,
      info,
    }
  }

  function closeWhoisModal() {
    whoisState.value = {
      ...whoisState.value,
      open: false,
    }
  }

  function parseLocalChannelId(channelId: string): { serverId: string, channelName: string } | null {
    const separator = channelId.indexOf('::')
    if (separator === -1) return null

    const serverId = channelId.slice(0, separator)
    const encoded = channelId.slice(separator + 2)
    if (!serverId || !encoded) return null

    return {
      serverId,
      channelName: `#${decodeURIComponent(encoded)}`,
    }
  }

  function toLocalChannelId(serverId: string, channelName: string): string {
    return `${serverId}::${encodeURIComponent(channelName.replace(/^#/, ''))}`
  }

  function applyChannelRename(serverId: string, oldChannelName: string, newChannelName: string) {
    const oldId = toLocalChannelId(serverId, oldChannelName)
    const newId = toLocalChannelId(serverId, newChannelName)
    if (oldId === newId) return

    const server = serversState.value.find(entry => entry.id === serverId)
    const channel = server?.channels.find(entry => entry.id === oldId)
    if (channel) {
      channel.id = newId
      channel.name = newChannelName.replace(/^#/, '')
      serversState.value = [...serversState.value]
    }

    messagesState.value = messagesState.value.map(message => (
      message.channelId === oldId
        ? { ...message, channelId: newId }
        : message
    ))

    for (const [localMessageId, meta] of Object.entries(ircMessageMetaByLocalId.value)) {
      if (meta.serverId !== serverId) continue
      if (meta.channelName !== oldChannelName) continue

      ircMessageMetaByLocalId.value[localMessageId] = {
        ...meta,
        channelName: newChannelName,
      }
    }

    if (activeChannelId.value === oldId) {
      activeChannelId.value = newId
    }

    const topic = channelTopicsById.value[oldId]
    if (topic !== undefined) {
      const { [oldId]: _drop, ...rest } = channelTopicsById.value
      channelTopicsById.value = {
        ...rest,
        [newId]: topic,
      }
      saveLocalJson(CHANNEL_TOPICS_KEY, channelTopicsById.value)
    }

    const settings = channelSettingsById.value[oldId]
    if (settings) {
      const { [oldId]: _drop, ...rest } = channelSettingsById.value
      channelSettingsById.value = {
        ...rest,
        [newId]: settings,
      }
    }

    const participation = channelParticipationById.value[oldId]
    if (participation) {
      const { [oldId]: _drop, ...rest } = channelParticipationById.value
      channelParticipationById.value = {
        ...rest,
        [newId]: participation,
      }
    }

    const serviceActivity = channelServiceActivityById.value[oldId]
    if (serviceActivity) {
      const { [oldId]: _drop, ...rest } = channelServiceActivityById.value
      channelServiceActivityById.value = {
        ...rest,
        [newId]: serviceActivity,
      }
    }

    let changedConfig = false
    ircServers.value = ircServers.value.map((config) => {
      if (config.id !== serverId) return config
      const nextChannels = config.channels.map((name) => (
        name.toLowerCase() === oldChannelName.toLowerCase() ? newChannelName : name
      ))
      changedConfig = true
      return {
        ...config,
        channels: uniqueChannelNames(nextChannels),
      }
    })

    if (changedConfig) {
      saveLocalJson(IRC_SERVERS_KEY, ircServers.value)
    }
  }

  function normalizeChannelName(value: string): string {
    const trimmed = value.trim()
    if (!trimmed) return '#general'
    return trimmed.startsWith('#') ? trimmed : `#${trimmed}`
  }

  function uniqueChannelNames(channels: string[]): string[] {
    return Array.from(new Set(channels.map(normalizeChannelName)))
  }

  function normalizeIrcConfig(input: IrcConnectInput): IrcServerConfig {
    const serverName = input.serverName.trim() || 'localhost'
    const displayName = input.name.trim() || serverName
    const idSeed = `${displayName}-${serverName}-${input.webSocketUrl.trim()}`
    const id = idSeed.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '') || `irc-${Date.now()}`
    const abbreviation = displayName
      .split(/\s+/)
      .filter(Boolean)
      .map(part => part[0])
      .join('')
      .slice(0, 2)
      .toUpperCase() || 'IR'

    return {
      id,
      name: displayName,
      abbreviation,
      webSocketUrl: input.webSocketUrl.trim(),
      serverName,
      nick: input.nick.trim() || dataSource.ircDefaults.nick,
      username: input.username.trim() || input.nick.trim() || dataSource.ircDefaults.username,
      realname: input.realname?.trim() || dataSource.ircDefaults.realname,
      serverPassword: input.serverPassword?.trim() || input.password?.trim() || undefined,
      servicesAccount: input.servicesAccount.trim() || input.nick.trim() || dataSource.ircDefaults.nick,
      servicesPassword: input.servicesPassword.trim(),
      autoOwnershipTake: Boolean(input.autoOwnershipTake),
      channels: uniqueChannelNames(input.channels.length ? input.channels : []),
    }
  }

  function ensureIrcUser(nick: string) {
    const normalizedNick = nick || 'unknown'
    const existing = usersState.value.find(user => user.username === normalizedNick)
    if (existing) return existing

    const created = {
      id: `irc-user-${normalizedNick.toLowerCase()}`,
      username: normalizedNick,
      avatar: normalizedNick.slice(0, 1).toUpperCase() || 'U',
      status: 'online' as const,
    }
    usersState.value = [...usersState.value, created]
    return created
  }

  function serverEndpointKey(webSocketUrl: string, serverName: string): string {
    return `${webSocketUrl.trim().toLowerCase()}::${serverName.trim().toLowerCase()}`
  }

  function ensureIrcServer(config: IrcServerConfig) {
    const existing = serversState.value.find(server => server.id === config.id)
    if (existing) {
      existing.name = config.name
      existing.abbreviation = config.abbreviation
      return existing
    }

    const created: Server = {
      id: config.id,
      name: config.name,
      abbreviation: config.abbreviation,
      channels: [],
    }

    serversState.value = [...serversState.value, created]
    return created
  }

  function ensureIrcChannel(serverId: string, channelName: string) {
    const server = serversState.value.find(entry => entry.id === serverId)
    if (!server) return toLocalChannelId(serverId, channelName)

    const localChannelId = toLocalChannelId(serverId, channelName)
    const existing = server.channels.find(channel => channel.id === localChannelId)
    if (existing) return existing.id

    const created: Channel = {
      id: localChannelId,
      name: channelName.replace(/^#/, ''),
      type: 'text',
      serverId,
      topic: '',
      modes: '+nt',
      members: [],
    }

    server.channels = [...server.channels, created]
    serversState.value = [...serversState.value]
    return created.id
  }

  function registerIrcEvent(server: IrcServerConfig, event: IrcIncomingMessageEvent) {
    if (!event.channelId.startsWith('#')) return

    const localChannelId = ensureIrcChannel(server.id, event.channelId)

    if (isServiceActivityAuthor(event.author)) {
      addChannelServiceActivity(localChannelId, event.author, event.content, event.timestamp)
      return
    }

    const author = ensureIrcUser(event.author)
    const existingLocalId = event.msgid ? ircLocalIdByMsgid.value[event.msgid] : undefined

    const replyLocalMessageId = event.replyToMsgid ? ircLocalIdByMsgid.value[event.replyToMsgid] : undefined
    const replyMessage = replyLocalMessageId ? getMessageById(replyLocalMessageId) : null

    if (event.reaction) {
      if (replyLocalMessageId) {
        applyReactionToMessage(replyLocalMessageId, event.reaction)
      } else if (event.replyToMsgid) {
        queuePendingReaction(event.replyToMsgid, event.reaction)
      }
      return
    }

    if (existingLocalId) {
      const index = messagesState.value.findIndex(message => message.id === existingLocalId)
      if (index >= 0) {
        const updated = {
          ...messagesState.value[index],
          content: event.content,
          timestamp: event.timestamp,
        }
        messagesState.value[index] = updated

        // Persist edit to IndexedDB
        // Deep clone to strip Proxy
        const plainUpdated = JSON.parse(JSON.stringify({
          ...updated,
          serverId: server.id,
          ircMsgid: event.msgid,
        }))
        messageDb.putMessage(plainUpdated as StoredMessage).catch(() => {})
      }
      return
    }

    const localMessageId = `irc-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    const newMessage: Message = {
      id: localMessageId,
      channelId: localChannelId,
      author,
      content: event.content,
      timestamp: event.timestamp,
      replyTo: replyMessage ? {
        messageId: replyMessage.id,
        authorUsername: replyMessage.author.username,
        content: replyMessage.content,
      } : undefined,
    }
    messagesState.value.push(newMessage)

    // Persist to IndexedDB
    // We must deep clone to strip Vue Proxies (author object is reactive), otherwise IDB throws DataCloneError
    const plainMessage = JSON.parse(JSON.stringify({
      ...newMessage,
      serverId: server.id,
      ircMsgid: event.msgid,
    }))

    messageDb.putMessage(plainMessage as StoredMessage).catch((err) => {
      console.error('[ircord] putMessage failed:', err)
    })

    ircMessageMetaByLocalId.value = {
      ...ircMessageMetaByLocalId.value,
      [localMessageId]: {
        serverId: server.id,
        channelName: event.channelId,
        msgid: event.msgid,
      },
    }

    if (event.msgid) {
      ircLocalIdByMsgid.value = {
        ...ircLocalIdByMsgid.value,
        [event.msgid]: localMessageId,
      }

      applyPendingReferences(event.msgid, localMessageId)

      // Update cursor for next reconnect
      messageDb.setLastMessageMeta(localChannelId, {
        msgid: event.msgid,
        timestamp: event.timestamp
      }).catch(() => {})
    }

    if (!replyMessage && event.replyToMsgid) {
      queuePendingReplyLink(event.replyToMsgid, localMessageId)
    }
  }

  function applyReactionToMessage(messageId: string, emoji: string) {
    const current = messageReactionsById.value[messageId] || []
    if (current.includes(emoji)) return

    messageReactionsById.value = {
      ...messageReactionsById.value,
      [messageId]: [...current, emoji],
    }
    saveLocalJson(MESSAGE_REACTIONS_KEY, messageReactionsById.value)
  }

  function queuePendingReplyLink(parentMsgid: string, childLocalId: string) {
    const current = pendingReplyLinksByParentMsgid.value[parentMsgid] || []
    if (current.includes(childLocalId)) return

    pendingReplyLinksByParentMsgid.value = {
      ...pendingReplyLinksByParentMsgid.value,
      [parentMsgid]: [...current, childLocalId],
    }
  }

  function queuePendingReaction(parentMsgid: string, emoji: string) {
    const current = pendingReactionsByParentMsgid.value[parentMsgid] || []
    if (current.includes(emoji)) return

    pendingReactionsByParentMsgid.value = {
      ...pendingReactionsByParentMsgid.value,
      [parentMsgid]: [...current, emoji],
    }
  }

  function applyPendingReferences(parentMsgid: string, parentLocalId: string) {
    const pendingChildren = pendingReplyLinksByParentMsgid.value[parentMsgid] || []
    if (pendingChildren.length) {
      const parent = getMessageById(parentLocalId)
      if (parent) {
        for (const childId of pendingChildren) {
          const index = messagesState.value.findIndex(message => message.id === childId)
          if (index === -1) continue

          const child = messagesState.value[index]
          messagesState.value[index] = {
            ...child,
            replyTo: {
              messageId: parent.id,
              authorUsername: parent.author.username,
              content: parent.content,
            },
          }
        }
      }

      const { [parentMsgid]: _ignore, ...rest } = pendingReplyLinksByParentMsgid.value
      pendingReplyLinksByParentMsgid.value = rest
    }

    const pendingReactions = pendingReactionsByParentMsgid.value[parentMsgid] || []
    if (pendingReactions.length) {
      for (const emoji of pendingReactions) {
        applyReactionToMessage(parentLocalId, emoji)
      }
      const { [parentMsgid]: _ignore, ...rest } = pendingReactionsByParentMsgid.value
      pendingReactionsByParentMsgid.value = rest
    }
  }

  function ircNamesToChannelMode(mode: NamesEntry['mode']): 'op' | 'voice' | 'regular' {
    if (mode === 'owner' || mode === 'admin' || mode === 'op') return 'op'
    if (mode === 'halfop' || mode === 'voice') return 'voice'
    return 'regular'
  }

  function channelModeRank(mode: ChannelMode): number {
    if (mode === 'op') return 2
    if (mode === 'voice') return 1
    return 0
  }

  function extractSimpleModeLetters(rawModes: string): Set<string> {
    const source = rawModes.trim()
    const modeToken = source.split(/\s+/).find(token => token.startsWith('+') || token.startsWith('-')) || ''
    return new Set(
      modeToken
        .replace(/[+-]/g, '')
        .split('')
        .filter(Boolean),
    )
  }

  function extractTopicText(msg: ReturnType<typeof parseIrcMessage>, topicParamIndex: number): string {
    if (msg.trailing) return msg.trailing
    return msg.params.slice(topicParamIndex).join(' ').trim()
  }

  function applyChannelModes(channelId: string, modes: string) {
    for (const server of serversState.value) {
      const channel = server.channels.find(entry => entry.id === channelId)
      if (!channel) continue
      channel.modes = modes
      serversState.value = [...serversState.value]
      return
    }
  }

  function upsertMemberInChannel(channelId: string, username: string, mode: ChannelMode) {
    for (const server of serversState.value) {
      const channel = server.channels.find(entry => entry.id === channelId)
      if (!channel) continue

      const index = channel.members.findIndex(entry => entry.user.username === username)
      if (index === -1) {
        channel.members = [...channel.members, { user: ensureIrcUser(username), mode }]
      } else {
        channel.members[index] = {
          ...channel.members[index],
          mode,
        }
      }

      serversState.value = [...serversState.value]
      return
    }
  }

  function removeMemberFromChannel(channelId: string, username: string) {
    for (const server of serversState.value) {
      const channel = server.channels.find(entry => entry.id === channelId)
      if (!channel) continue

      channel.members = channel.members.filter(entry => entry.user.username !== username)
      serversState.value = [...serversState.value]
      return
    }
  }

  function applyChannelUserModeDelta(channelId: string, modeChange: string, args: string[]) {
    if (!modeChange.startsWith('+') && !modeChange.startsWith('-')) return
    let add = true
    let argIndex = 0

    for (const flag of modeChange) {
      if (flag === '+') {
        add = true
        continue
      }
      if (flag === '-') {
        add = false
        continue
      }

      if (flag !== 'o' && flag !== 'v') continue
      const target = args[argIndex]
      argIndex += 1
      if (!target) continue

      if (!add) {
        upsertMemberInChannel(channelId, target, 'regular')
        continue
      }

      upsertMemberInChannel(channelId, target, flag === 'o' ? 'op' : 'voice')
    }
  }

  function handleIrcRawLine(serverId: string, line: string) {
    const msg = parseIrcMessage(line)
    const selfNick = ircServerStatus.value[serverId]?.nick || currentUser.value.username

    if (msg.command === 'RENAME') {
      const oldChannelName = msg.params[0] || ''
      const newChannelName = msg.params[1] || ''
      if (!oldChannelName.startsWith('#') || !newChannelName.startsWith('#')) return
      applyChannelRename(serverId, oldChannelName, newChannelName)
      return
    }

    if (msg.command === 'NOTICE') {
      const author = parseNickname(msg.prefix)
      if (!isServiceActivityAuthor(author)) return

      const rawText = msg.trailing || ''
      const channelMatch = rawText.match(/#[-_A-Za-z0-9]+/)
      if (!channelMatch) return

      const channelName = channelMatch[0]
      const localChannelId = ensureIrcChannel(serverId, channelName)
      const timestamp = msg.tags.time
        ? new Date(msg.tags.time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
        : new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

      addChannelServiceActivity(localChannelId, author, rawText, timestamp)
      return
    }

    if (msg.command === 'TOPIC') {
      const channelName = msg.params[0] || ''
      if (!channelName.startsWith('#')) return
      setLocalChannelTopic(toLocalChannelId(serverId, channelName), extractTopicText(msg, 1))
      return
    }

    if (msg.command === '332') {
      const channelName = msg.params[1] || ''
      if (!channelName.startsWith('#')) return
      setLocalChannelTopic(toLocalChannelId(serverId, channelName), extractTopicText(msg, 2))
      return
    }

    if (msg.command === '324') {
      const channelName = msg.params[1] || ''
      if (!channelName.startsWith('#')) return
      applyChannelModes(toLocalChannelId(serverId, channelName), msg.params[2] || '')
      return
    }

    if (msg.command === 'MODE') {
      const channelName = msg.params[0] || ''
      if (!channelName.startsWith('#')) return
      const modeChange = msg.params[1] || ''
      const modeArgs = msg.params.slice(2)
      applyChannelUserModeDelta(toLocalChannelId(serverId, channelName), modeChange, modeArgs)
      return
    }

    if (msg.command === 'JOIN') {
      const channelName = msg.trailing || msg.params[0] || ''
      if (!channelName.startsWith('#')) return
      const nick = parseNickname(msg.prefix)
      const localChannelId = toLocalChannelId(serverId, channelName)
      if (nick.toLowerCase() === selfNick.toLowerCase()) {
        setChannelParticipation(localChannelId, {
          joined: true,
          reason: null,
          detail: null,
          canRejoin: true,
        })
      }
      upsertMemberInChannel(localChannelId, nick, 'regular')
      return
    }

    if (msg.command === 'PART') {
      const channelName = msg.params[0] || ''
      if (!channelName.startsWith('#')) return
      const nick = parseNickname(msg.prefix)
      const localChannelId = toLocalChannelId(serverId, channelName)
      if (nick.toLowerCase() === selfNick.toLowerCase()) {
        setChannelParticipation(localChannelId, {
          joined: false,
          reason: 'part',
          detail: msg.trailing || 'You left the channel',
          canRejoin: true,
        })
        addChannelSystemNotice(localChannelId, `You left ${channelName}${msg.trailing ? ` (${msg.trailing})` : ''}`)
      }
      removeMemberFromChannel(localChannelId, nick)
      return
    }

    if (msg.command === 'KICK') {
      const channelName = msg.params[0] || ''
      if (!channelName.startsWith('#')) return
      const target = msg.params[1] || ''
      const localChannelId = toLocalChannelId(serverId, channelName)
      const reason = msg.trailing || ''
      if (target.toLowerCase() === selfNick.toLowerCase()) {
        const kicker = parseNickname(msg.prefix) || 'unknown'
        const looksLikeBan = /ban/i.test(reason)
        setChannelParticipation(localChannelId, {
          joined: false,
          reason: looksLikeBan ? 'ban' : 'kick',
          detail: reason || null,
          canRejoin: !looksLikeBan,
        })
        addChannelSystemNotice(
          localChannelId,
          looksLikeBan
            ? `You were banned and kicked from ${channelName} by ${kicker}${reason ? `: ${reason}` : ''}`
            : `You were kicked from ${channelName} by ${kicker}${reason ? `: ${reason}` : ''}`,
        )
      }

      removeMemberFromChannel(localChannelId, target)
      return
    }

    if (msg.command === '474') {
      const channelName = msg.params[1] || ''
      if (!channelName.startsWith('#')) return
      const localChannelId = toLocalChannelId(serverId, channelName)
      setChannelParticipation(localChannelId, {
        joined: false,
        reason: 'ban',
        detail: msg.trailing || 'Cannot join channel (+b)',
        canRejoin: false,
      })
      addChannelSystemNotice(localChannelId, `You are banned from ${channelName}${msg.trailing ? `: ${msg.trailing}` : ''}`)
    }
  }

  async function syncIrcChannelMembersFromNames(serverId: string, channelName: string): Promise<void> {
    const adapter = ircAdapters.get(serverId)
    if (!adapter) return

    try {
      const names = await adapter.requestNames(channelName)
      const localChannelId = toLocalChannelId(serverId, channelName)
      const server = serversState.value.find(entry => entry.id === serverId)
      const channel = server?.channels.find(entry => entry.id === localChannelId)
      if (!channel) return

      channel.members = names.map(entry => ({
        user: ensureIrcUser(entry.nick),
        mode: ircNamesToChannelMode(entry.mode),
      }))
      serversState.value = [...serversState.value]
    } catch (err) {
      console.warn('[ircord] NAMES failed for', channelName, err)
    }
  }

  function isEligibleForOwnership(localChannelId: string, nick: string): boolean {
    const channel = serversState.value
      .flatMap(server => server.channels)
      .find(entry => entry.id === localChannelId)
    if (!channel) return false
    if (channel.members.length !== 1) return false

    const onlyMember = channel.members[0]
    return onlyMember.user.username.toLowerCase() === nick.toLowerCase() && onlyMember.mode === 'op'
  }

  async function ensureErgoServicesIdentity(config: IrcServerConfig, adapter: IrcAdapter): Promise<boolean> {
    const account = (config.servicesAccount || config.username || config.nick || '').trim()
    const secret = (config.servicesPassword || '').trim()
    if (!account || !secret) return false

    adapter.sendServiceCommand('NickServ', `IDENTIFY ${account} ${secret}`)
    const identified = await adapter.waitForServiceNotice(
      'NickServ',
      /identified|already.+identified|logged\s+in|recognized/i,
      6000,
      /invalid|incorrect|failed|denied|not\s+registered|unknown\s+account|does\s+not\s+exist/i,
    )
    if (identified) return true

    adapter.sendServiceCommand('NickServ', `IDENTIFY ${secret}`)
    const identifiedByPassphrase = await adapter.waitForServiceNotice(
      'NickServ',
      /identified|already.+identified|logged\s+in|recognized/i,
      6000,
      /invalid|incorrect|failed|denied|not\s+registered|unknown\s+account|does\s+not\s+exist/i,
    )
    if (identifiedByPassphrase) return true

    adapter.sendServiceCommand('NickServ', `REGISTER ${secret}`)
    const registered = await adapter.waitForServiceNotice(
      'NickServ',
      /registered|account.+created|success|verify|activation/i,
      7000,
      /already\s+registered|already\s+exists|invalid|failed|denied/i,
    )
    if (!registered) return false

    adapter.sendServiceCommand('NickServ', `IDENTIFY ${secret}`)
    const identifiedAfterRegister = await adapter.waitForServiceNotice(
      'NickServ',
      /identified|logged\s+in|recognized/i,
      5000,
      /invalid|incorrect|failed|denied/i,
    )
    if (identifiedAfterRegister) return true

    adapter.sendServiceCommand('NickServ', `IDENTIFY ${account} ${secret}`)
    return adapter.waitForServiceNotice(
      'NickServ',
      /identified|logged\s+in|recognized/i,
      5000,
      /invalid|incorrect|failed|denied/i,
    )
  }

  async function claimChannelOwnership(adapter: IrcAdapter, channelName: string): Promise<boolean> {
    adapter.sendServiceCommand('ChanServ', `REGISTER ${channelName}`)
    const registered = await adapter.waitForServiceNotice(
      'ChanServ',
      /registered|now\s+registered|success|added/i,
      7000,
      /already\s+registered|already\s+exists|denied|failed|invalid/i,
    )

    adapter.sendServiceCommand('ChanServ', `OP ${channelName}`)
    const opped = await adapter.waitForServiceNotice(
      'ChanServ',
      /opped|operator\s+status|mode\s+\+o|you\s+are\s+now\s+an\s+operator/i,
      5000,
      /access\s+denied|not\s+registered|failed|invalid/i,
    )

    return registered || opped
  }

  async function maybeHandleChannelOwnership(
    config: IrcServerConfig,
    adapter: IrcAdapter,
    channelName: string,
    localChannelId: string,
    servicesReady: boolean,
  ): Promise<void> {
    if (!servicesReady) return
    if (!isEligibleForOwnership(localChannelId, adapter.currentNick)) return

    if (config.autoOwnershipTake) {
      await claimChannelOwnership(adapter, channelName)
      return
    }

    const promptKey = `${config.id}:${channelName.toLowerCase()}`
    if (ownershipPromptedByChannel.value[promptKey]) return

    ownershipPromptedByChannel.value = {
      ...ownershipPromptedByChannel.value,
      [promptKey]: true,
    }

    if (!import.meta.client || typeof window.confirm !== 'function') return

    const accepted = window.confirm(
      `You are currently the only user in ${channelName} and have operator. Register and claim ownership via ChanServ now?`,
    )
    if (!accepted) return
    await claimChannelOwnership(adapter, channelName)
  }

  async function connectAndSyncIrcServer(config: IrcServerConfig): Promise<boolean> {
    if (ircAdapters.has(config.id)) return true

    const adapter = new IrcAdapter({
      nick: config.nick,
      username: config.username,
      realname: config.realname,
      password: config.serverPassword || config.password,
      serverName: config.serverName,
      transport: new WebSocketIrcTransport({ url: config.webSocketUrl }),
    })

    try {
      await adapter.connect()
    } catch (error) {
      adapter.disconnect()
      console.error('[ircord] IRC connect failed', {
        serverId: config.id,
        webSocketUrl: config.webSocketUrl,
        serverName: config.serverName,
        error,
      })
      return false
    }

    ensureIrcServer(config)

    // Initialize server status
    ircServerStatus.value = {
      ...ircServerStatus.value,
      [config.id]: {
        rawLog: [...adapter.rawLog],
        latencyMs: adapter.latencyMs,
        nick: adapter.currentNick,
        userModes: adapter.userModes,
        capabilities: adapter.negotiatedCapabilities,
        connectedAt: adapter.connectedAt,
      },
    }

    // Stream raw log updates
    adapter.onRawLine((entry) => {
      const status = ircServerStatus.value[config.id]
      if (status) {
        ircServerStatus.value = {
          ...ircServerStatus.value,
          [config.id]: {
            ...status,
            rawLog: [...adapter.rawLog],
            latencyMs: adapter.latencyMs,
            nick: adapter.currentNick,
            userModes: adapter.userModes,
          },
        }
      }

      if (entry.direction === 'in') {
        handleIrcRawLine(config.id, entry.line)
      }
    })

    adapter.onMessage(event => {
      registerIrcEvent(config, event)
    })

    ircAdapters.set(config.id, adapter)

    const servicesReady = await ensureErgoServicesIdentity(config, adapter)

    for (const channelName of config.channels) {
      // Create the channel entry FIRST so it appears in the sidebar immediately
      const localChannelId = ensureIrcChannel(config.id, channelName)

      try {
        await adapter.joinChannel(channelName)

        await syncIrcChannelMembersFromNames(config.id, channelName)
        await maybeHandleChannelOwnership(config, adapter, channelName, localChannelId, servicesReady)

        // Load persisted messages from IndexedDB first
        await loadChannelMessagesFromDb(localChannelId)

        // Fetch CHATHISTORY — use stored cursor to only get new messages
        try {
          const meta = await messageDb.getLastMessageMeta(localChannelId)
          const since = meta?.lastMsgid ? { msgid: meta.lastMsgid } : undefined
          const history = await adapter.requestHistory(channelName, 100, since)

          let newestMsgid: string | null = null
          let newestTimestamp: string | null = null

          for (const msg of history) {
            // Dedup: skip if we already have this msgid
            if (msg.msgid && ircLocalIdByMsgid.value[msg.msgid]) continue

            registerIrcEvent(config, {
              channelId: msg.channelId,
              author: msg.author,
              content: msg.content,
              msgid: msg.msgid,
              replyToMsgid: msg.replyToMsgid,
              reaction: msg.reaction,
              tags: msg.tags,
              timestamp: msg.timestamp,
            })

            if (msg.msgid) newestMsgid = msg.msgid
            newestTimestamp = msg.timestamp
          }

          // Update cursor for next reconnect
          if (newestMsgid || newestTimestamp) {
            await messageDb.setLastMessageMeta(localChannelId, {
              msgid: newestMsgid || meta?.lastMsgid || null,
              timestamp: newestTimestamp,
            })
          }
        } catch (err) {
          console.warn('[ircord] CHATHISTORY failed for', channelName, err)
        }

        await refreshChannelSettings(localChannelId)
      } catch (err) {
        console.warn('[ircord] JOIN failed for', channelName, err)
      }
    }

    return true
  }

  return {
    isIrcMode,
    currentUser,
    activeServerId,
    activeChannelId,
    serversList,
    dmsList,
    channelsList,
    customGroupsForActiveServer,
    collapsedSectionsForActiveServer,
    gifAutoplay,
    gifFavorites,
    mediaScalePercent,
    recentReactions,
    messagesList,
    activeServer,
    activeChannel,
    activeChannelSettings,
    activeChannelParticipation,
    activeChannelServiceActivity,
    whoisState,
    channelMembersList,
    activeServerStatus,
    getChannelTopic,
    canEditChannelTopic,
    canManageChannelBans,
    canModerateChannelMember,
    setActiveServer,
    setActiveChannel,
    setChannelTopic,
    setChannelModes,
    renameChannel,
    kickChannelMember,
    banChannelMember,
    addChannelBan,
    refreshChannelSettings,
    selectServerStatus,
    createCustomGroup,
    toggleCustomGroupCollapsed,
    deleteCustomGroup,
    assignItemToGroup,
    moveGroupItemToIndex,
    moveCustomGroupToIndex,
    getItemGroupId,
    toggleSectionCollapsed,
    setGifAutoplay,
    setMediaScalePercent,
    addRecentReaction,
    toggleGifFavorite,
    getMessageReactions,
    toggleMessageReaction,
    replyTargetMessage,
    setReplyTarget,
    editMockMessage,
    applyNavigationState,
    resetSelection,
    sendMockMessage,
    connectIrcServer,
    initializeIrcConnections,
    joinIrcChannel,
    rejoinActiveChannel,
    openWhoisModal,
    closeWhoisModal,
  }
})

function loadLocalJson<T>(key: string, fallback: T): T {
  if (!import.meta.client) return fallback

  const raw = localStorage.getItem(key)
  if (!raw) return fallback

  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function saveLocalJson<T>(key: string, value: T) {
  if (!import.meta.client) return
  localStorage.setItem(key, JSON.stringify(value))
}

function defaultCollapsedSections(): Record<SidebarSectionKey, boolean> {
  return {
    dms: false,
    channels: false,
    groups: false,
  }
}
