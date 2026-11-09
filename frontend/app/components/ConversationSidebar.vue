<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { serverViewId } from '../stores/chat'
import { draftCount } from '../../shared/utils/drafts'
import { collapsedItems } from '../../shared/utils/sidebar'
import { isConversationPath } from '../../shared/utils/workspace-routes'
const emit = defineEmits<{ search: []; drafts: []; join: []; close: [] }>()
const chat = useChatStore()
const route = useRoute()
const navigation = useConversationNavigation()
const notifications = useNotificationStore()
const menus = useMenus()
const layout = useLayoutStore()
const actions = useWorkspaceActions()
const inbox = useInboxStore()
const networkMenu = ref<HTMLButtonElement | null>(null)
const sidebar = ref<HTMLElement | null>(null)
function toggleSidebar() {
  layout.set('showSidebar', !layout.state.showSidebar)
  nextTick(() =>
    sidebar.value
      ?.querySelector<HTMLButtonElement>(
        layout.state.showSidebar ? '.sidebar-collapse' : '.sidebar-restore',
      )
      ?.focus(),
  )
}
const markRead = useMarkRead()
const inNetwork = computed(
  () =>
    chat.data?.conversations.filter(
      (c) =>
        c.networkId === chat.networkId &&
        (!layout.state.unreadOnly || c.unreadCount || c.mentionCount || c.id === chat.activeId),
    ) ?? [],
)
const unreadHere = computed(() => markRead.unread(chat.networkId).length)
const name = (c: Conversation) =>
  c.kind === 'direct' ? (chat.directPartner(c)?.displayName ?? c.name) : c.name
const muted = (c: Conversation) => notifications.level(c) === 'none'
const byName = (a: Conversation, b: Conversation) =>
  name(a).localeCompare(name(b), undefined, { numeric: true, sensitivity: 'base' })
const favorites = computed(() => inNetwork.value.filter((c) => c.favorite).sort(byName))
const channels = computed(() =>
  inNetwork.value.filter((c) => c.kind === 'channel' && !c.favorite).sort(byName),
)
const directs = computed(() => inNetwork.value.filter((c) => c.kind === 'direct' && !c.favorite))
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
const serverId = computed(() => serverViewId(chat.networkId))
const drafts = computed(() => draftCount(chat.drafts))
</script>
<template>
  <aside
    ref="sidebar"
    class="conversation-sidebar relative flex min-w-0 flex-col bg-sidebar max-[899px]:[&_.resize-handle]:hidden"
    aria-label="Conversations"
  >
    <CollapsedSidebar
      v-if="!layout.state.showSidebar"
      @expand="toggleSidebar"
    />
    <div
      class="sidebar-content min-h-0 flex-1 overflow-y-auto px-2 pb-4 pt-3 [touch-action:pan-y_pinch-zoom]"
    >
      <div class="flex items-center gap-1">
        <ConversationLink
          class="min-w-0 flex-1"
          :active="route.path === '/search'"
          aria-label="Search messages"
          @click="emit('search')"
          ><AppIcon
            name="search"
            :size="15"
          /><span class="flex-1 truncate">Search</span></ConversationLink
        >
        <UiButton
          variant="ghost"
          size="icon-sm"
          class="sidebar-collapse shrink-0 max-[899px]:hidden"
          aria-label="Hide conversations"
          title="Hide conversations"
          @click="toggleSidebar"
          ><AppIcon
            name="sidebarClose"
            :size="17"
        /></UiButton>
        <UiButton
          variant="ghost"
          size="icon-sm"
          class="navigation-close hidden shrink-0 max-[899px]:inline-flex"
          aria-label="Close conversations"
          @click="emit('close')"
          ><AppIcon
            name="close"
            :size="18"
        /></UiButton>
      </div>
      <ConversationLink
        :active="route.path === '/inbox'"
        :aria-label="inbox.unread ? `Inbox, ${inbox.unread} unread` : 'Inbox'"
        @click="actions.inbox()"
        ><AppIcon
          name="inbox"
          :size="15"
        /><span class="flex-1 truncate">Inbox</span
        ><span
          v-if="inbox.unread"
          class="min-w-4 rounded-full bg-selection px-1 text-center text-[10px] font-semibold text-primary"
          >{{ inbox.unread > 99 ? '99+' : inbox.unread }}</span
        ></ConversationLink
      >
      <ConversationLink
        :active="route.path === '/saved'"
        aria-label="Saved messages"
        @click="actions.saved()"
        ><AppIcon
          name="bookmark"
          :size="15"
        /><span class="flex-1 truncate">Saved</span></ConversationLink
      >
      <ConversationLink
        class="drafts-link"
        :active="route.path === '/drafts'"
        :aria-label="drafts ? `Drafts, ${drafts} on this device` : 'Drafts, none on this device'"
        @click="emit('drafts')"
        ><AppIcon
          name="pencil"
          :size="15"
        /><span class="flex-1 truncate">Drafts</span
        ><span
          v-if="drafts"
          class="draft-count ml-auto min-w-4 rounded-full bg-secondary px-1 text-center text-[10px] font-semibold text-muted-foreground"
          aria-hidden="true"
          >{{ drafts }}</span
        ></ConversationLink
      >
      <ConversationLink
        :active="route.path === '/links'"
        aria-label="Links"
        @click="actions.links()"
        ><AppIcon
          name="link"
          :size="15"
        /><span class="flex-1 truncate">Links</span></ConversationLink
      >
      <div class="mt-1 flex items-center">
        <ConversationLink
          class="server-link min-w-0 flex-1"
          :active="route.path.startsWith('/server') && chat.activeId === serverId"
          @click="navigation.open(serverId)"
          @contextmenu="chat.network && menus.network($event, chat.network)"
          ><AppIcon
            name="terminal"
            :size="15"
          /><span class="flex-1 truncate">Server</span></ConversationLink
        >
        <button
          ref="networkMenu"
          class="flex size-7 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
          aria-label="Network options"
          aria-haspopup="menu"
          @click="chat.network && menus.network(networkMenu!, chat.network)"
        >
          <AppIcon
            name="more"
            :size="15"
          />
        </button>
      </div>
      <div class="sidebar-filter mt-2 flex flex-wrap items-center justify-between gap-1">
        <button
          class="filter-toggle flex h-7 items-center gap-1 rounded-sm px-2 text-[length:var(--ui-small)] text-muted-foreground hover:bg-accent hover:text-foreground [&.active]:bg-selection [&.active]:text-primary [@media(pointer:coarse)]:min-h-[44px]"
          :class="{ active: layout.state.unreadOnly }"
          :aria-pressed="layout.state.unreadOnly"
          title="Show only conversations with unread messages"
          @click="layout.set('unreadOnly', !layout.state.unreadOnly)"
        >
          <AppIcon
            name="bell"
            :size="12"
          />Unread only
        </button>
        <UiButton
          v-if="unreadHere"
          variant="link"
          size="text"
          class="px-2 py-0.5 text-[length:var(--ui-caption)]"
          @click="markRead.network(chat.networkId)"
          >Mark all read</UiButton
        >
      </div>
      <section
        v-for="section in sections"
        :key="section.id"
        class="sidebar-section"
        :class="{ 'is-collapsed': section.collapsed }"
      >
        <div
          class="sidebar-section-heading mb-0.5 mt-4 flex h-7 items-center justify-between pl-2 pr-1 text-[length:var(--ui-caption)] font-semibold uppercase tracking-[0.02em] text-muted-foreground [@media(pointer:coarse)]:min-h-[44px]"
        >
          <button
            class="section-toggle -ml-1.5 flex h-full min-w-0 items-center gap-0.5 rounded-sm pl-0.5 pr-1 text-inherit [font:inherit] tracking-[inherit] [text-transform:inherit] hover:text-foreground [&>svg]:shrink-0 [&>svg]:transition-transform [&>svg]:duration-150 aria-[expanded=false]:[&>svg]:-rotate-90 [&>span]:truncate"
            :aria-expanded="!section.collapsed"
            :aria-controls="`sidebar-section-${section.id}`"
            @click="layout.toggleSection(chat.networkId, section.id)"
          >
            <AppIcon
              name="down"
              :size="11"
            /><span>{{ section.label }}</span>
          </button>
          <AppTooltip
            v-if="section.action"
            :text="section.action === 'join' ? 'Join a channel' : 'Message someone'"
            side="top"
            ><UiButton
              variant="ghost"
              size="icon-sm"
              class="size-[22px]"
              :aria-label="section.action === 'join' ? 'Join a channel' : 'Message someone'"
              @click="section.action === 'join' ? emit('join') : emit('search')"
              ><AppIcon
                name="plus"
                :size="14" /></UiButton
          ></AppTooltip>
        </div>
        <div :id="`sidebar-section-${section.id}`">
          <ConversationLink
            v-for="c in section.shown"
            :key="c.id"
            :active="isConversationPath(route.path) && c.id === chat.activeId"
            :unread="c.unreadCount > 0 && !muted(c)"
            :direct="c.kind === 'direct'"
            :muted="muted(c)"
            @click="navigation.open(c.id)"
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
              :size="15"
            />
            <span class="conversation-name min-w-0 flex-1 truncate">{{ name(c) }}</span>
            <span
              v-if="c.mentionCount"
              class="mention-badge flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-rail"
              :aria-label="`${c.mentionCount} mentions`"
              >{{ c.mentionCount }}</span
            >
            <i
              v-else-if="c.unreadCount && !muted(c)"
              class="unread-dot mr-0.5 size-1.5 rounded-full bg-foreground"
              aria-label="Unread messages"
            />
            <AppIcon
              v-else-if="muted(c)"
              name="muted"
              :size="12"
              class="text-faint"
              aria-label="Muted"
            />
          </ConversationLink>
          <p
            v-if="!section.items.length && !section.collapsed && section.id === 'directs'"
            class="sidebar-empty px-2 py-1 text-[length:var(--ui-caption)] text-muted-foreground"
          >
            No conversations yet.
          </p>
        </div>
      </section>
    </div>
    <slot />
  </aside>
</template>
