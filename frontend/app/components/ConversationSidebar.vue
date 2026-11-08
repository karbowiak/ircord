<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { serverViewId } from '../stores/chat'
import { draftCount } from '../../shared/utils/drafts'
import { ariaShortcut, isMac, shortcutLabel } from '../../shared/utils/shortcuts'
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
const favorites = computed(() => inNetwork.value.filter((c) => c.favorite))
const channels = computed(() => inNetwork.value.filter((c) => c.kind === 'channel' && !c.favorite))
const directs = computed(() => inNetwork.value.filter((c) => c.kind === 'direct' && !c.favorite))
const sections = computed(() =>
  [
    { id: 'favorites', label: 'Favorites', items: favorites.value },
    { id: 'channels', label: 'Channels', items: channels.value, action: 'join' as const },
    { id: 'directs', label: 'Direct messages', items: directs.value, action: 'search' as const },
  ].filter((s) => s.items.length || s.action),
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
    class="conversation-sidebar"
    aria-label="Conversations"
  >
    <button
      ref="heading"
      class="network-heading"
      aria-haspopup="menu"
      :title="`${chat.network?.host}:${chat.network?.port}`"
      @click="chat.network && menus.network(heading!, chat.network)"
      @contextmenu="chat.network && menus.network($event, chat.network)"
    >
      <strong>{{ chat.network?.name }}</strong>
      <i
        v-if="chat.network?.status !== 'connected'"
        class="status-dot"
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
    <div class="sidebar-content">
      <div class="sidebar-filter">
        <button
          class="filter-toggle"
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
        class="conversation-link server-link"
        :class="{ 'is-selected': chat.activeId === serverId }"
        :aria-current="chat.activeId === serverId ? 'page' : undefined"
        @click="chat.activate(serverId)"
        @contextmenu="chat.network && menus.network($event, chat.network)"
      >
        <AppIcon
          name="terminal"
          :size="17"
        /><span class="conversation-name">Server</span>
      </button>
      <button
        class="conversation-link drafts-link"
        :aria-label="drafts ? `Drafts, ${drafts} on this device` : 'Drafts, none on this device'"
        @click="emit('drafts')"
      >
        <AppIcon
          name="pencil"
          :size="17"
        /><span class="conversation-name">Drafts</span
        ><span
          v-if="drafts"
          class="draft-count"
          aria-hidden="true"
          >{{ drafts }}</span
        >
      </button>
      <section
        v-for="section in sections"
        :key="section.id"
        class="sidebar-section"
      >
        <div class="sidebar-section-heading">
          <span>{{ section.label }}</span>
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
        <button
          v-for="c in section.items"
          :key="c.id"
          class="conversation-link"
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
          <span class="conversation-name">{{ name(c) }}</span>
          <span
            v-if="c.mentionCount"
            class="mention-badge"
            :aria-label="`${c.mentionCount} mentions`"
            >{{ c.mentionCount }}</span
          ><i
            v-else-if="c.unreadCount && !muted(c)"
            class="unread-dot"
            aria-label="Unread messages"
          /><AppIcon
            v-else-if="muted(c)"
            name="muted"
            :size="13"
            class="muted-icon"
            aria-label="Muted"
          />
        </button>
        <p
          v-if="!section.items.length && section.id === 'directs'"
          class="sidebar-empty"
        >
          No conversations yet.
        </p>
      </section>
    </div>
    <div class="self-panel">
      <button
        class="self-profile"
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
