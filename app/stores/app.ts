import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Server, Channel, DMChannel, Message, ChannelMember } from '~/types'
import { useMockData } from '~/composables/useMockData'

const LAST_CHANNEL_KEY = 'ircord-last-channel-by-server'
const GROUPS_KEY = 'ircord-custom-groups-by-server'
const COLLAPSED_SECTIONS_KEY = 'ircord-collapsed-sections-by-server'
const SETTINGS_KEY = 'ircord-client-settings'

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

type ClientSettings = {
  gifAutoplay: boolean
}

export const useAppStore = defineStore('app', () => {
  const { users, servers, dms, messages } = useMockData()
  const messagesState = ref<Message[]>([...messages])

  const currentUser = ref(users[0])
  const activeServerId = ref<string | null>(null)
  const activeChannelId = ref<string | null>(null)
  const settings = ref<ClientSettings>({
    gifAutoplay: true,
    ...loadLocalJson<ClientSettings>(SETTINGS_KEY, { gifAutoplay: true }),
  })
  const lastChannelByServer = ref<ServerChannelMemory>(loadLocalJson(LAST_CHANNEL_KEY, {}))
  const customGroupsByServer = ref<GroupsByServer>(loadLocalJson(GROUPS_KEY, {}))
  const collapsedSectionsByServer = ref<CollapsedSections>(loadLocalJson(COLLAPSED_SECTIONS_KEY, {}))

  const serversList = computed(() => servers)
  const dmsList = computed(() => dms)
  const channelsList = computed(() => {
    if (!activeServerId.value) return []
    return servers.find(s => s.id === activeServerId.value)?.channels || []
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
  const messagesList = computed(() => {
    if (!activeChannelId.value) return []
    return messagesState.value.filter(m => m.channelId === activeChannelId.value)
  })

  const activeServer = computed(() =>
    servers.find(s => s.id === activeServerId.value) || null
  )
  const activeChannel = computed((): Channel | DMChannel | null => {
    if (!activeChannelId.value) return null
    const channel = channelsList.value.find(c => c.id === activeChannelId.value)
    if (channel) return channel
    return dms.find(d => d.id === activeChannelId.value) || null
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
    return dms.some(dm => dm.id === channelId)
  }

  function isChannelInServer(channelId: string, serverId: string): boolean {
    return servers.some(server =>
      server.id === serverId && server.channels.some(channel => channel.id === channelId)
    )
  }

  function setActiveServer(serverId: string) {
    if (!servers.some(server => server.id === serverId)) return

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

    if (activeServerId.value && inServer) {
      lastChannelByServer.value = {
        ...lastChannelByServer.value,
        [activeServerId.value]: channelId,
      }
      saveLocalJson(LAST_CHANNEL_KEY, lastChannelByServer.value)
    }
  }

  function selectServerStatus() {
    activeChannelId.value = null
  }

  function createCustomGroup(name: string) {
    if (!activeServerId.value) return

    const trimmed = name.trim()
    if (!trimmed) return

    const current = customGroupsByServer.value[activeServerId.value] || []
    const next: CustomGroup[] = [
      ...current,
      {
        id: `group-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        name: trimmed,
        collapsed: false,
        itemIds: [],
      },
    ]

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [activeServerId.value]: next,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function toggleCustomGroupCollapsed(groupId: string) {
    if (!activeServerId.value) return
    const current = customGroupsByServer.value[activeServerId.value] || []

    const next = current.map((group) => {
      if (group.id !== groupId) return group
      return { ...group, collapsed: !group.collapsed }
    })

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [activeServerId.value]: next,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function deleteCustomGroup(groupId: string) {
    if (!activeServerId.value) return
    const current = customGroupsByServer.value[activeServerId.value] || []
    const next = current.filter(group => group.id !== groupId)

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [activeServerId.value]: next,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function assignItemToGroup(itemId: string, groupId: string | null) {
    if (!activeServerId.value) return

    const current = customGroupsByServer.value[activeServerId.value] || []

    const withoutItem = current.map(group => ({
      ...group,
      itemIds: group.itemIds.filter(id => id !== itemId),
    }))

    const next = groupId
      ? withoutItem.map(group => {
        if (group.id !== groupId) return group
        if (group.itemIds.includes(itemId)) return group
        return { ...group, itemIds: [...group.itemIds, itemId] }
      })
      : withoutItem

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [activeServerId.value]: next,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function moveGroupItemToIndex(groupId: string, itemId: string, targetIndex: number) {
    if (!activeServerId.value) return

    const current = customGroupsByServer.value[activeServerId.value] || []
    const next = current.map((group) => {
      if (group.id !== groupId) return group

      const from = group.itemIds.indexOf(itemId)
      if (from === -1) return group

      const bounded = Math.max(0, Math.min(targetIndex, group.itemIds.length - 1))
      if (from === bounded) return group

      const reordered = [...group.itemIds]
      const [item] = reordered.splice(from, 1)
      reordered.splice(bounded, 0, item)

      return { ...group, itemIds: reordered }
    })

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [activeServerId.value]: next,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function moveCustomGroupToIndex(groupId: string, targetIndex: number) {
    if (!activeServerId.value) return

    const current = [...(customGroupsByServer.value[activeServerId.value] || [])]
    const from = current.findIndex(group => group.id === groupId)
    if (from === -1) return

    const bounded = Math.max(0, Math.min(targetIndex, current.length - 1))
    if (from === bounded) return

    const [group] = current.splice(from, 1)
    current.splice(bounded, 0, group)

    customGroupsByServer.value = {
      ...customGroupsByServer.value,
      [activeServerId.value]: current,
    }

    saveLocalJson(GROUPS_KEY, customGroupsByServer.value)
  }

  function getItemGroupId(itemId: string): string | null {
    const group = customGroupsForActiveServer.value.find(entry => entry.itemIds.includes(itemId))
    return group?.id || null
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

  function applyNavigationState(serverId: string | null, channelId: string | null) {
    const fallbackServerId = servers[0]?.id || null
    const nextServerId = serverId && servers.some(server => server.id === serverId)
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
    activeServerId.value = servers[0]?.id || null
    activeChannelId.value = null
  }

  function sendMockMessage(content: string): boolean {
    const text = content.trim()
    if (!text || !activeChannelId.value) return false

    messagesState.value.push({
      id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      channelId: activeChannelId.value,
      author: currentUser.value,
      content: text,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    })

    return true
  }

  return {
    currentUser,
    activeServerId,
    activeChannelId,
    serversList,
    dmsList,
    channelsList,
    customGroupsForActiveServer,
    collapsedSectionsForActiveServer,
    gifAutoplay,
    messagesList,
    activeServer,
    activeChannel,
    channelMembersList,
    setActiveServer,
    setActiveChannel,
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
    applyNavigationState,
    resetSelection,
    sendMockMessage,
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
