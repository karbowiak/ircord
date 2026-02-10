<template>
  <div class="channel-sidebar">
    <div ref="quickActionsEl" class="header">
      <span class="server-name">{{ headerTitle }}</span>
      <button type="button" class="header-action" title="Quick actions" @click="toggleQuickActionsMenu">+</button>

      <div v-if="quickActionsMenuOpen" class="quick-menu">
        <button type="button" class="quick-item" @click="startQuickAction('group')">New Group</button>
        <button type="button" class="quick-item" @click="startQuickAction('join')">Join Channel</button>
        <button type="button" class="quick-item" @click="startQuickAction('dm')">Send DM To...</button>
      </div>
    </div>

    <div class="channel-list">
      <div v-if="quickActionMode" class="quick-editor">
        <input
          ref="quickInputEl"
          v-model="quickActionValue"
          type="text"
          class="quick-input"
          :placeholder="quickActionPlaceholder"
          @input="onQuickActionInput"
          @keydown="onQuickActionKeydown"
          @keydown.esc.prevent="cancelQuickAction"
        >
        <button type="button" class="mini-action" @click="submitQuickAction">Go</button>

        <div v-if="quickActionSuggestions.length" class="quick-suggestions">
          <button
            v-for="(item, idx) in quickActionSuggestions"
            :key="item"
            type="button"
            class="quick-suggestion"
            :class="{ active: idx === quickActionSuggestionIndex }"
            @mousedown.prevent
            @click="applyQuickSuggestion(item)"
          >
            {{ item }}
          </button>
        </div>
      </div>

      <div v-if="actionNotice" class="action-notice">{{ actionNotice }}</div>

      <div class="section">
        <div class="section-header">Server</div>
        <div class="status-meta">{{ serverStatusText }}</div>
        <button
          type="button"
          class="row status-row"
          :class="{ active: appStore.activeChannelId === null }"
          @click="appStore.selectServerStatus()"
        >
          <span class="status-pill" :class="serverStatusClass" />
          <span class="row-label">Server Status</span>
        </button>
      </div>

      <div class="section">
        <div v-if="!groupedEntries.length" class="group-empty-root">No custom groups yet. Use + to create one.</div>

        <div
          v-for="(group, groupIndex) in groupedEntries"
          :key="group.id"
          class="group-card"
          :class="{ 'drop-highlight': isGroupDropActive(group.id), 'drop-insert-before': isGroupInsertTarget(groupIndex) }"
          @dragover.prevent="setDropMarker(dragging?.origin === 'group-list' ? 'group-list' : 'group', group.id, groupIndex)"
          @drop.prevent="onGroupContainerDrop(group.id, groupIndex)"
        >
          <div class="group-header">
            <button
              type="button"
              class="drag-handle"
              title="Reorder group"
              draggable="true"
              @dragstart.stop="onDragStart($event, group.id, 'group-list')"
              @dragend.stop="onDragEnd"
            >
              ⋮⋮
            </button>
            <button type="button" class="group-title" @click="appStore.toggleCustomGroupCollapsed(group.id)">
              <span>{{ group.collapsed ? '▸' : '▾' }}</span>
              <span>{{ group.name }}</span>
            </button>
            <button type="button" class="mini-action" title="Delete group" @click="appStore.deleteCustomGroup(group.id)">×</button>
          </div>

          <div v-if="!group.collapsed" class="group-body">
            <div
              v-for="(entry, index) in group.items"
              :key="`${group.id}-${entry.id}`"
              class="row-wrap"
              :class="{ 'drop-target': isRowDropTarget('group-item', group.id, index) }"
              @dragover.prevent.stop="setDropMarker('group-item', group.id, index)"
              @drop.prevent.stop="onGroupDropAt(group.id, index)"
            >
              <button
                type="button"
                class="row"
                :class="{ active: appStore.activeChannelId === entry.id, unread: entry.unread }"
                draggable="true"
                @dragstart.stop="onDragStart($event, entry.id, 'group', group.id)"
                @dragend.stop="onDragEnd"
                @click="appStore.setActiveChannel(entry.id)"
              >
                <span v-if="entry.kind === 'channel'" class="prefix">#</span>
                <span v-else class="status-pill" :class="entry.status || 'offline'" />
                <span class="row-label">{{ entry.label }}</span>
              </button>

              <button type="button" class="mini-action" title="Remove from group" @click.stop="appStore.assignItemToGroup(entry.id, null)">↩</button>
            </div>

            <div v-if="!group.items.length" class="group-empty">Drop channels or DMs here</div>
          </div>
        </div>
      </div>

      <div class="section" :class="{ 'drop-highlight': isSectionDropActive('dms') }" @dragover.prevent="setDropMarker('dms')" @drop.prevent="onUngroupDrop">
        <button type="button" class="section-toggle" @click="appStore.toggleSectionCollapsed('dms')">
          <span>{{ appStore.collapsedSectionsForActiveServer.dms ? '▸' : '▾' }}</span>
          <span>Direct Messages</span>
        </button>
        <div v-if="!appStore.collapsedSectionsForActiveServer.dms">
          <div
            v-for="(entry, index) in dmEntries"
            :key="entry.id"
            class="row-wrap"
            :class="{ 'drop-target': isRowDropTarget('dms-item', undefined, index) }"
            @dragover.prevent.stop="setDropMarker('dms-item', undefined, index)"
            @drop.prevent.stop="onUngroupDrop"
          >
            <button
              type="button"
              class="row"
              :class="{ active: appStore.activeChannelId === entry.id, unread: entry.unread }"
              draggable="true"
              @dragstart.stop="onDragStart($event, entry.id, 'other')"
              @dragend.stop="onDragEnd"
              @click="appStore.setActiveChannel(entry.id)"
            >
              <span class="status-pill" :class="entry.status || 'offline'" />
              <span class="row-label">{{ entry.label }}</span>
            </button>
          </div>
        </div>
      </div>

      <div class="section" :class="{ 'drop-highlight': isSectionDropActive('channels') }" @dragover.prevent="setDropMarker('channels')" @drop.prevent="onUngroupDrop">
        <button type="button" class="section-toggle" @click="appStore.toggleSectionCollapsed('channels')">
          <span>{{ appStore.collapsedSectionsForActiveServer.channels ? '▸' : '▾' }}</span>
          <span>Channels</span>
        </button>
        <div v-if="!appStore.collapsedSectionsForActiveServer.channels">
          <div
            v-for="(entry, index) in channelEntries"
            :key="entry.id"
            class="row-wrap"
            :class="{ 'drop-target': isRowDropTarget('channels-item', undefined, index) }"
            @dragover.prevent.stop="setDropMarker('channels-item', undefined, index)"
            @drop.prevent.stop="onUngroupDrop"
          >
            <button
              type="button"
              class="row"
              :class="{ active: appStore.activeChannelId === entry.id, unread: entry.unread }"
              draggable="true"
              @dragstart.stop="onDragStart($event, entry.id, 'other')"
              @dragend.stop="onDragEnd"
              @click="appStore.setActiveChannel(entry.id)"
            >
              <span class="prefix">#</span>
              <span class="row-label">{{ entry.label }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <div class="spacer" />
    <UserPanel />
  </div>
</template>

<script setup lang="ts">
import UserPanel from '~/components/layout/UserPanel.vue'

type SidebarEntry = {
  id: string
  label: string
  kind: 'channel' | 'dm'
  unread?: boolean
  status?: 'online' | 'away' | 'offline'
}

type DragPayload = {
  id: string
  origin: 'group' | 'group-list' | 'other'
  groupId?: string
}

type DropMarker = {
  zone: 'group' | 'group-item' | 'group-list' | 'dms' | 'channels' | 'dms-item' | 'channels-item'
  groupId?: string
  index?: number
} | null

const appStore = useAppStore()
const dragging = ref<DragPayload | null>(null)
const dropMarker = ref<DropMarker>(null)

const quickActionsEl = ref<HTMLDivElement>()
const quickInputEl = ref<HTMLInputElement>()
const quickActionsMenuOpen = ref(false)
const quickActionMode = ref<'group' | 'join' | 'dm' | null>(null)
const quickActionValue = ref('')
const quickActionSuggestionIndex = ref(0)
const actionNotice = ref('')
let noticeTimeout: ReturnType<typeof setTimeout> | null = null

const headerTitle = computed(() => appStore.activeServer?.name || 'Server')
const serverStatusClass = computed(() => appStore.activeServerId ? 'online' : 'offline')
const serverStatusText = computed(() => appStore.activeServerId ? 'CONNECTED' : 'OFFLINE')

const quickActionPlaceholder = computed(() => {
  if (quickActionMode.value === 'group') return 'Group name (e.g. Work)'
  if (quickActionMode.value === 'join') return '#channel (e.g. #linux)'
  if (quickActionMode.value === 'dm') return 'nickname (e.g. alice)'
  return ''
})

const seenUsernames = computed(() => {
  const names = new Set<string>()

  for (const dm of appStore.dmsList) {
    names.add(dm.recipient.username)
  }

  for (const channel of appStore.channelsList) {
    for (const member of channel.members) {
      names.add(member.user.username)
    }
  }

  return Array.from(names).sort((a, b) => a.localeCompare(b))
})

const quickActionSuggestions = computed(() => {
  const query = quickActionValue.value.trim().toLowerCase()
  if (!query) return []

  if (quickActionMode.value === 'join') {
    return appStore.channelsList
      .map(channel => `#${channel.name}`)
      .filter(name => name.toLowerCase().includes(query))
      .slice(0, 8)
  }

  if (quickActionMode.value === 'dm') {
    return seenUsernames.value
      .filter(name => name.toLowerCase().includes(query.replace(/^@/, '')))
      .slice(0, 8)
  }

  return []
})

watch(quickActionSuggestions, (items) => {
  if (!items.length) {
    quickActionSuggestionIndex.value = 0
    return
  }

  if (quickActionSuggestionIndex.value >= items.length) {
    quickActionSuggestionIndex.value = 0
  }
})

function setActionNotice(message: string) {
  actionNotice.value = message
  if (noticeTimeout) clearTimeout(noticeTimeout)
  noticeTimeout = setTimeout(() => {
    actionNotice.value = ''
  }, 2400)
}

function resolveEntry(id: string): SidebarEntry | null {
  const channel = appStore.channelsList.find(item => item.id === id)
  if (channel) {
    return { id, label: channel.name, kind: 'channel', unread: channel.unread }
  }

  const dm = appStore.dmsList.find(item => item.id === id)
  if (dm) {
    return {
      id,
      label: dm.recipient.username,
      kind: 'dm',
      unread: dm.unread,
      status: dm.recipient.status,
    }
  }

  return null
}

const groupedItemIds = computed(() => {
  const ids = new Set<string>()
  for (const group of appStore.customGroupsForActiveServer) {
    for (const id of group.itemIds) ids.add(id)
  }
  return ids
})

const dmEntries = computed(() => {
  return appStore.dmsList
    .filter(dm => !groupedItemIds.value.has(dm.id))
    .map(dm => ({
      id: dm.id,
      label: dm.recipient.username,
      kind: 'dm' as const,
      unread: dm.unread,
      status: dm.recipient.status,
    }))
})

const channelEntries = computed(() => {
  return appStore.channelsList
    .filter(channel => !groupedItemIds.value.has(channel.id))
    .map(channel => ({
      id: channel.id,
      label: channel.name,
      kind: 'channel' as const,
      unread: channel.unread,
    }))
})

const groupedEntries = computed(() => {
  return appStore.customGroupsForActiveServer.map(group => ({
    id: group.id,
    name: group.name,
    collapsed: group.collapsed,
    items: group.itemIds.flatMap((id) => {
      const entry = resolveEntry(id)
      return entry ? [entry] : []
    }),
  }))
})

function toggleQuickActionsMenu() {
  quickActionsMenuOpen.value = !quickActionsMenuOpen.value
}

function startQuickAction(mode: 'group' | 'join' | 'dm') {
  quickActionMode.value = mode
  quickActionValue.value = ''
  quickActionSuggestionIndex.value = 0
  quickActionsMenuOpen.value = false

  nextTick(() => quickInputEl.value?.focus())
}

function cancelQuickAction() {
  quickActionMode.value = null
  quickActionValue.value = ''
  quickActionSuggestionIndex.value = 0
}

function onQuickActionInput() {
  quickActionSuggestionIndex.value = 0
}

function applyQuickSuggestion(value: string) {
  quickActionValue.value = value
  quickInputEl.value?.focus()
}

function onQuickActionKeydown(event: KeyboardEvent) {
  if (quickActionSuggestions.value.length > 0) {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      quickActionSuggestionIndex.value = (quickActionSuggestionIndex.value + 1) % quickActionSuggestions.value.length
      return
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault()
      quickActionSuggestionIndex.value = (quickActionSuggestionIndex.value - 1 + quickActionSuggestions.value.length) % quickActionSuggestions.value.length
      return
    }

    if (event.key === 'Tab') {
      event.preventDefault()
      const picked = quickActionSuggestions.value[quickActionSuggestionIndex.value]
      if (picked) {
        applyQuickSuggestion(picked)
      }
      return
    }
  }

  if (event.key === 'Enter') {
    event.preventDefault()
    submitQuickAction()
  }
}

function submitQuickAction() {
  const raw = quickActionValue.value.trim()
  if (!raw) return

  if (quickActionMode.value === 'group') {
    appStore.createCustomGroup(raw)
    setActionNotice(`Group '${raw}' created`)
    cancelQuickAction()
    return
  }

  if (quickActionMode.value === 'join') {
    const normalized = raw.replace(/^#/, '').toLowerCase()
    const match = appStore.channelsList.find(channel => channel.name.toLowerCase() === normalized)

    if (match) {
      appStore.setActiveChannel(match.id)
      setActionNotice(`Joined #${match.name} (mock)`)
    } else {
      setActionNotice(`IRC note: #${normalized} is not in this mock server list`) 
    }

    cancelQuickAction()
    return
  }

  if (quickActionMode.value === 'dm') {
    const normalized = raw.replace(/^@/, '').toLowerCase()
    const match = appStore.dmsList.find(dm => dm.recipient.username.toLowerCase() === normalized)

    if (match) {
      appStore.setActiveChannel(match.id)
      setActionNotice(`Opened DM with ${match.recipient.username}`)
    } else {
      setActionNotice(`IRC note: user '${normalized}' is not in seen users`) 
    }

    cancelQuickAction()
  }
}

function onClickOutside(event: MouseEvent) {
  if (!quickActionsEl.value) return
  if (quickActionsEl.value.contains(event.target as Node)) return
  quickActionsMenuOpen.value = false
}

function setDropMarker(zone: NonNullable<DropMarker>['zone'], groupId?: string, index?: number) {
  dropMarker.value = { zone, groupId, index }
}

function clearDropMarker() {
  dropMarker.value = null
}

function isSectionDropActive(section: 'dms' | 'channels') {
  return dropMarker.value?.zone === section
}

function isGroupDropActive(groupId: string) {
  return dropMarker.value?.zone === 'group' && dropMarker.value?.groupId === groupId
}

function isRowDropTarget(zone: 'group-item' | 'dms-item' | 'channels-item', groupId?: string, index?: number) {
  return dropMarker.value?.zone === zone
    && dropMarker.value?.groupId === groupId
    && dropMarker.value?.index === index
}

function isGroupInsertTarget(index: number) {
  return dropMarker.value?.zone === 'group-list' && dropMarker.value?.index === index
}

function onDragStart(event: DragEvent, id: string, origin: DragPayload['origin'], groupId?: string) {
  dragging.value = { id, origin, groupId }

  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', id)
  }
}

function onDragEnd() {
  dragging.value = null
  clearDropMarker()
}

function onGroupDrop(groupId: string) {
  if (!dragging.value) return
  appStore.assignItemToGroup(dragging.value.id, groupId)
  onDragEnd()
}

function onGroupContainerDrop(groupId: string, index: number) {
  if (!dragging.value) return

  if (dragging.value.origin === 'group-list') {
    appStore.moveCustomGroupToIndex(dragging.value.id, index)
    onDragEnd()
    return
  }

  onGroupDrop(groupId)
}

function onGroupDropAt(groupId: string, index: number) {
  if (!dragging.value) return
  appStore.assignItemToGroup(dragging.value.id, groupId)
  appStore.moveGroupItemToIndex(groupId, dragging.value.id, index)
  onDragEnd()
}

function onUngroupDrop() {
  if (!dragging.value) return
  if (dragging.value.origin === 'group') {
    appStore.assignItemToGroup(dragging.value.id, null)
  }
  onDragEnd()
}

onMounted(() => {
  window.addEventListener('mousedown', onClickOutside, { capture: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('mousedown', onClickOutside, { capture: true })
  if (noticeTimeout) clearTimeout(noticeTimeout)
})
</script>

<style scoped>
.channel-sidebar { width: 240px; height: 100vh; background-color: var(--bg-dark); display: flex; flex-direction: column; }
.header { height: 48px; padding: 0 12px 0 16px; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border-dark); box-shadow: 0 1px 0 rgba(0,0,0,.2); flex-shrink: 0; position: relative; }
.server-name { font-size: 15px; font-weight: 600; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.header-action { border: 0; border-radius: 6px; background: rgba(255,255,255,.06); color: var(--text-body); font-family: var(--font-mono); font-size: 14px; width: 24px; height: 24px; cursor: pointer; }
.header-action:hover { background: rgba(255,255,255,.13); }

.quick-menu { position: absolute; right: 10px; top: calc(100% + 6px); width: 170px; border-radius: 8px; border: 1px solid rgba(255,255,255,.08); background: rgba(34, 36, 43, .98); box-shadow: 0 14px 28px rgba(0,0,0,.35); overflow: hidden; z-index: 30; }
.quick-item { width: 100%; border: 0; background: transparent; color: var(--text-body); font-family: var(--font-mono); font-size: 12px; text-align: left; padding: 8px 10px; cursor: pointer; }
.quick-item:hover { background: var(--bg-hover); }

.channel-list { flex: 1; overflow-y: auto; padding-bottom: 8px; }
.quick-editor { margin: 10px 8px 4px 8px; position: relative; display: flex; gap: 6px; }
.quick-input { flex: 1; min-width: 0; border: 1px solid rgba(255,255,255,.08); border-radius: 6px; background: var(--bg-input); color: var(--text-body); padding: 6px 8px; font-family: var(--font-mono); font-size: 12px; outline: none; }
.mini-action { border: 0; border-radius: 4px; background: transparent; color: var(--text-faint); cursor: pointer; font-size: 11px; padding: 2px 6px; }
.mini-action:hover { background-color: var(--bg-hover); color: var(--text-primary); }
.quick-suggestions { position: absolute; left: 0; right: 32px; top: calc(100% + 4px); border: 1px solid rgba(255,255,255,.08); border-radius: 8px; background: rgba(34,36,43,.98); box-shadow: 0 8px 18px rgba(0,0,0,.32); overflow: hidden; z-index: 25; max-height: 180px; overflow-y: auto; }
.quick-suggestion { width: 100%; border: 0; background: transparent; color: var(--text-body); font-family: var(--font-mono); font-size: 12px; text-align: left; padding: 7px 8px; cursor: pointer; }
.quick-suggestion:hover, .quick-suggestion.active { background: rgba(88,101,242,.2); }
.action-notice { margin: 6px 16px 0 16px; font-family: var(--font-mono); font-size: 11px; color: var(--text-faint); }

.section { margin-top: 12px; transition: background-color .12s ease; }
.section.drop-highlight { background: rgba(88,101,242,.08); }
.section-header { padding: 0 8px 4px 16px; font-family: var(--font-mono); font-size: 11px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: .05em; }
.section-toggle { border: 0; background: transparent; color: var(--text-muted); font-family: var(--font-mono); font-size: 11px; text-transform: uppercase; letter-spacing: .05em; display: flex; gap: 6px; align-items: center; padding: 0 8px 4px 16px; cursor: pointer; }
.status-meta { margin: 0 16px 5px 16px; font-family: var(--font-mono); font-size: 10px; color: var(--accent-green); letter-spacing: .06em; }

.row-wrap { position: relative; display: flex; align-items: center; gap: 4px; padding-right: 6px; transition: background-color .08s ease; }
.row-wrap.drop-target { background: rgba(88,101,242,.10); }
.row-wrap.drop-target::before {
  content: '';
  position: absolute;
  left: 8px;
  right: 8px;
  top: -1px;
  height: 2px;
  border-radius: 999px;
  background: rgba(128, 151, 255, .95);
}
.row { flex: 1; display: flex; align-items: center; min-width: 0; padding: 5px 8px; margin: 1px 8px; border: 0; border-radius: 4px; background: transparent; color: var(--text-muted); cursor: pointer; transition: background-color .1s ease, color .1s ease; }
.row:hover { background-color: var(--bg-hover); color: var(--text-body); }
.row.active { background-color: var(--bg-active); color: var(--text-primary); }
.row.unread .row-label { color: var(--text-primary); font-weight: 500; }
.prefix { font-family: var(--font-mono); font-size: 16px; margin-right: 6px; opacity: .55; }
.row-label { font-family: var(--font-mono); font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.status-row { align-items: center; }
.status-pill { width: 8px; height: 8px; border-radius: 50%; margin-right: 8px; flex-shrink: 0; background-color: var(--text-faint); }
.status-pill.online { background-color: var(--accent-green); }
.status-pill.away { background-color: var(--accent-orange); }
.status-pill.offline { background-color: var(--text-faint); }

.group-card { margin: 4px 8px; border: 1px solid rgba(255,255,255,.08); border-radius: 7px; background: rgba(0,0,0,.14); }
.group-card.drop-highlight { border-color: rgba(88,101,242,.5); box-shadow: inset 0 0 0 1px rgba(88,101,242,.25); }
.group-card.drop-insert-before { box-shadow: inset 0 0 0 1px rgba(88,101,242,.22); }
.group-card.drop-insert-before::before {
  content: '';
  display: block;
  height: 2px;
  margin: -1px 6px 2px 6px;
  border-radius: 999px;
  background: rgba(128, 151, 255, .95);
}
.group-header { display: flex; align-items: center; justify-content: space-between; padding: 4px 4px 0 0; }
.drag-handle { border: 0; background: transparent; color: var(--text-faint); cursor: grab; font-size: 11px; padding: 3px 4px; margin-left: 4px; border-radius: 4px; }
.drag-handle:hover { color: var(--text-primary); background: rgba(255,255,255,.06); }
.drag-handle:active { cursor: grabbing; }
.group-title { border: 0; background: transparent; color: var(--text-muted); font-family: var(--font-mono); font-size: 12px; display: flex; gap: 6px; align-items: center; padding: 4px 8px; cursor: pointer; }
.group-body { padding-bottom: 4px; }
.group-empty, .group-empty-root { padding: 8px 16px; font-family: var(--font-mono); font-size: 11px; color: var(--text-faint); }

.spacer { flex: 0; }
</style>
