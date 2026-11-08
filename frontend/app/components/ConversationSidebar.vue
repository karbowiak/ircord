<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { serverViewId } from '../stores/chat'
import { draftCount } from '../../shared/utils/drafts'
import { collapsedItems } from '../../shared/utils/sidebar'
import { ariaShortcut, isMac, shortcutLabel } from '../../shared/utils/shortcuts'
import { useQuery } from '@pinia/colada'
import { administrationAccessQuery } from '../queries/administration'
const emit = defineEmits<{
  search: []
  drafts: []
  join: []
  settings: []
  profile: [id: string]
}>()
const chat = useChatStore()
const notifications = useNotificationStore()
const menus = useMenus()
const mac = isMac(navigator.platform)
const heading = ref<HTMLButtonElement | null>(null)
const layout = useLayoutStore()
const actions = useWorkspaceActions()
/** Only server owners see the Administration entry; the backend enforces it either way. */
const administration = useQuery(administrationAccessQuery)
const markRead = useMarkRead()
/** With “unread only”, the open conversation stays listed so you don't lose your place. */
const inNetwork = computed(
  () =>
    chat.data?.conversations.filter(
      (c) =>
        c.networkId === chat.networkId &&
        (!layout.state.unreadOnly || c.unreadCount || c.mentionCount || c.id === chat.activeId),
    ) ?? [],
)
const unreadHere = computed(() => markRead.unread(chat.networkId).length)
/** Numbers first, then A–Z ignoring case, with “#2” before “#10”. */
const byName = (a: Conversation, b: Conversation) =>
  name(a).localeCompare(name(b), undefined, { numeric: true, sensitivity: 'base' })
const favorites = computed(() => inNetwork.value.filter((c) => c.favorite).sort(byName))
const channels = computed(() =>
  inNetwork.value.filter((c) => c.kind === 'channel' && !c.favorite).sort(byName),
)
const directs = computed(() => inNetwork.value.filter((c) => c.kind === 'direct' && !c.favorite))
/** A collapsed section still lists the open conversation and those that want attention. */
const sections = computed(() =>
  [
    { id: 'favorites', label: 'Favorites', items: favorites.value },
    { id: 'channels', label: 'Channels', items: channels.value, action: 'join' as const },
    { id: 'directs', label: 'Direct messages', items: directs.value, action: 'search' as const },
  ]
    .filter((s) => s.items.length || s.action)
    .map((s) => {
      const collapsed = layout.collapsed(chat.networkId, s.id)
      return {
        ...s,
        collapsed,
        shown: collapsed ? collapsedItems(s.items, chat.activeId, muted) : s.items,
      }
    }),
)
const name = (c: Conversation) =>
  c.kind === 'direct' ? (chat.directPartner(c)?.displayName ?? c.name) : c.name
const muted = (c: Conversation) => notifications.level(c) === 'none'
const serverId = computed(() => serverViewId(chat.networkId))
/**
 * Drafts across every network on this device. The entry is always there, so typing the first
 * character of a message never shifts the conversation list.
 */
const drafts = computed(() => draftCount(chat.drafts))
</script>
<template>
  <aside
    class="conversation-sidebar relative flex flex-col bg-sidebar min-w-0"
    aria-label="Conversations"
  >
    <button
      ref="heading"
      class="network-heading h-[var(--header-height)] shrink-0 w-full pt-0 pr-3 pb-0 pl-4 flex items-center gap-2 text-left border-b [border-bottom-style:solid] border-b-border text-[15px]"
      aria-haspopup="menu"
      :title="`${chat.network?.host}:${chat.network?.port}`"
      @click="chat.network && menus.network(heading!, chat.network)"
      @contextmenu="chat.network && menus.network($event, chat.network)"
    >
      <strong>{{ chat.network?.name }}</strong>
      <i
        v-if="chat.network?.status !== 'connected'"
        class="status-dot inline-block w-[7px] h-[7px] bg-success rounded-full shrink-0"
        :class="`is-${chat.network?.status ?? 'disconnected'}`"
        :title="
          {
            connected: 'Connected',
            connecting: 'Connecting…',
            error: chat.network?.statusDetail ?? 'Connection failed',
            disconnected: 'Disconnected',
          }[chat.network?.status ?? 'disconnected']
        "
      />
      <AppIcon
        name="down"
        :size="16"
      />
    </button>
    <div
      class="sidebar-content flex-1 min-h-0 overflow-y-auto pt-2.5 pr-2 pb-4 pl-2 [touch-action:pan-y_pinch-zoom]"
    >
      <div class="sidebar-filter flex items-center justify-between gap-1.5 mt-0.5 mr-0 mb-0 ml-0">
        <button
          class="filter-toggle inline-flex items-center gap-[5px] h-6 py-0 px-2 rounded-[12px] text-[12px] text-faint"
          :class="{ active: layout.state.unreadOnly }"
          :aria-pressed="layout.state.unreadOnly"
          title="Show only conversations with unread messages"
          @click="layout.set('unreadOnly', !layout.state.unreadOnly)"
        >
          <AppIcon
            name="bell"
            :size="13"
          />Unread only
        </button>
        <button
          v-if="unreadHere"
          class="text-button"
          @click="markRead.network(chat.networkId)"
        >
          Mark all read
        </button>
      </div>
      <button
        class="conversation-link server-link w-full flex items-center gap-2 min-h-[var(--row-height)] my-[1px] mx-0 rounded-sm py-0 px-2 text-muted-foreground text-[14px] text-left mt-1"
        :class="{ 'is-selected': chat.activeId === serverId }"
        :aria-current="chat.activeId === serverId ? 'page' : undefined"
        @click="chat.activate(serverId)"
        @contextmenu="chat.network && menus.network($event, chat.network)"
      >
        <AppIcon
          name="terminal"
          :size="17"
        /><span class="conversation-name overflow-hidden text-ellipsis whitespace-nowrap flex-1"
          >Server</span
        >
      </button>
      <button
        class="conversation-link w-full flex items-center gap-2 min-h-[var(--row-height)] my-[1px] mx-0 rounded-sm py-0 px-2 text-muted-foreground text-[14px] text-left drafts-link"
        :aria-label="drafts ? `Drafts, ${drafts} on this device` : 'Drafts, none on this device'"
        @click="emit('drafts')"
      >
        <AppIcon
          name="pencil"
          :size="17"
        /><span class="conversation-name overflow-hidden text-ellipsis whitespace-nowrap flex-1"
          >Drafts</span
        ><span
          v-if="drafts"
          class="draft-count ml-auto min-w-4.5 py-0 px-[5px] rounded-[9px] bg-secondary text-muted-foreground text-[11px] font-[700] leading-[18px] text-center"
          aria-hidden="true"
          >{{ drafts }}</span
        >
      </button>
      <section
        v-for="section in sections"
        :key="section.id"
        class="sidebar-section"
        :class="{ 'is-collapsed': section.collapsed }"
      >
        <div
          class="sidebar-section-heading flex items-center justify-between h-7 mt-3.5 mr-0 mb-0.5 ml-0 pt-0 pr-1 pb-0 pl-2 text-faint text-[11px] font-[600] tracking-[0.05em] uppercase"
        >
          <button
            class="section-toggle flex items-center gap-0.5 min-w-0 h-full ml-[-6px] pt-0 pr-1 pb-0 pl-0.5 rounded-sm text-inherit [font:inherit] tracking-[inherit] [text-transform:inherit]"
            :aria-expanded="!section.collapsed"
            :aria-controls="`sidebar-section-${section.id}`"
            @click="layout.toggleSection(chat.networkId, section.id)"
          >
            <AppIcon
              name="down"
              :size="12"
            /><span>{{ section.label }}</span>
          </button>
          <button
            v-if="section.action"
            class="icon-button"
            :aria-label="section.action === 'join' ? 'Join a channel' : 'Message someone'"
            :title="section.action === 'join' ? 'Join a channel' : 'Message someone'"
            @click="section.action === 'join' ? emit('join') : emit('search')"
          >
            <AppIcon
              name="plus"
              :size="15"
            />
          </button>
        </div>
        <div :id="`sidebar-section-${section.id}`">
          <button
            v-for="c in section.shown"
            :key="c.id"
            class="conversation-link w-full flex items-center gap-2 min-h-[var(--row-height)] my-[1px] mx-0 rounded-sm py-0 px-2 text-muted-foreground text-[14px] text-left"
            :class="{
              'is-selected': c.id === chat.activeId,
              'has-unread': c.unreadCount > 0 && !muted(c),
              'is-direct': c.kind === 'direct',
              'is-muted': muted(c),
            }"
            :aria-current="c.id === chat.activeId ? 'page' : undefined"
            @click="chat.activate(c.id)"
            @contextmenu="menus.conversation($event, c)"
          >
            <UserAvatar
              v-if="c.kind === 'direct'"
              :member="chat.directPartner(c)"
              size="tiny"
              presence
            />
            <AppIcon
              v-else
              name="hash"
              :size="17"
            />
            <span
              class="conversation-name overflow-hidden text-ellipsis whitespace-nowrap flex-1"
              >{{ name(c) }}</span
            >
            <span
              v-if="c.mentionCount"
              class="mention-badge bg-destructive text-rail text-[11px] min-w-4.5 h-4.5 py-0 px-[5px] flex items-center justify-center rounded-[9px] font-[700]"
              :aria-label="`${c.mentionCount} mentions`"
              >{{ c.mentionCount }}</span
            ><i
              v-else-if="c.unreadCount && !muted(c)"
              class="unread-dot w-[7px] h-[7px] bg-foreground rounded-full mr-0.5"
              aria-label="Unread messages"
            /><AppIcon
              v-else-if="muted(c)"
              name="muted"
              :size="13"
              class="muted-icon text-faint"
              aria-label="Muted"
            />
          </button>
          <p
            v-if="!section.items.length && !section.collapsed && section.id === 'directs'"
            class="sidebar-empty text-[12px] text-faint py-1 px-2"
          >
            No conversations yet.
          </p>
        </div>
      </section>
    </div>
    <div
      class="self-panel h-12 shrink-0 mt-0 mr-2 mb-[max(12px,_calc(env(safe-area-inset-bottom)_+_6px))] ml-2 py-0 px-1 flex items-center gap-1 bg-field rounded-md shadow-[inset_0_0_0_1px_var(--border)]"
    >
      <button
        class="self-profile flex-1 flex gap-2 items-center text-left py-1 px-1.5 rounded-sm min-w-0"
        aria-label="Open your profile"
        @click="emit('profile', 'you')"
        @contextmenu="chat.data && menus.member($event, chat.data.user)"
      >
        <UserAvatar
          :member="chat.self"
          size="small"
          presence
        /><span
          ><strong>{{ chat.self?.displayName }}</strong
          ><small
            >@{{ chat.network?.nick ?? chat.data?.user.nick
            }}{{ chat.data?.user.status === 'away' ? ' · away' : '' }}</small
          ></span
        >
      </button>
      <button
        v-if="administration.data.value?.owner"
        class="icon-button"
        aria-label="Administration"
        title="Administration"
        @click="actions.administration()"
      >
        <AppIcon
          name="shield"
          :size="18"
        />
      </button>
      <button
        class="icon-button"
        aria-label="User settings"
        :title="`Settings (${shortcutLabel('settings', mac)})`"
        :aria-keyshortcuts="ariaShortcut('settings', mac)"
        @click="emit('settings')"
      >
        <AppIcon
          name="settings"
          :size="18"
        />
      </button>
    </div>
    <slot />
  </aside>
</template>
