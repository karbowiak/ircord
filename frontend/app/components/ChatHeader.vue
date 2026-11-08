<script setup lang="ts">
import { stripFormatting } from '../../shared/utils/irc'

/** The bar above the conversation: what's open, its topic, and conversation actions. */
defineProps<{ title?: string; membersVisible: boolean }>()
const emit = defineEmits<{
  openNav: []
  toggleMembers: []
  toggleRail: []
  toggleSidebar: []
}>()
const layout = useLayoutStore()
const chat = useChatStore()
const menus = useMenus()
const actions = useWorkspaceActions()
const inbox = useInboxStore()
const onlineCount = computed(
  () => chat.activeMemberIds.filter((id) => chat.member(id)?.status === 'online').length,
)
function contextMenu(event: MouseEvent) {
  if (chat.serverView) chat.network && menus.network(event, chat.network)
  else if (chat.active) menus.conversation(event, chat.active)
}
function openDetails() {
  const c = chat.active
  if (!c) return
  if (c.kind === 'channel') actions.channelSettings(c.id)
  else actions.profile(chat.directPartner(c)?.id ?? '')
}
</script>
<template>
  <header
    class="chat-header flex items-center h-[var(--header-height)] pt-0 pr-2 pb-0 pl-4 gap-2 border-b [border-bottom-style:solid] border-b-border shrink-0 min-w-0"
    @contextmenu="contextMenu"
  >
    <!-- Shown only where each panel is docked (workspace.css); drawers use the menu button. -->
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="rail-toggle"
      :aria-label="layout.state.showRail ? 'Hide networks' : 'Show networks'"
      :title="layout.state.showRail ? 'Hide networks' : 'Show networks'"
      :aria-pressed="layout.state.showRail"
      @click="emit('toggleRail')"
    >
      <AppIcon
        name="server"
        :size="18"
      />
    </UiButton>
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="sidebar-toggle"
      :aria-label="layout.state.showSidebar ? 'Hide conversations' : 'Show conversations'"
      :title="layout.state.showSidebar ? 'Hide conversations' : 'Show conversations'"
      :aria-pressed="layout.state.showSidebar"
      @click="emit('toggleSidebar')"
    >
      <AppIcon
        :name="layout.state.showSidebar ? 'sidebarClose' : 'sidebarOpen'"
        :size="18"
      />
    </UiButton>
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="mobile-menu hidden"
      aria-label="Open conversations"
      @click="emit('openNav')"
    >
      <AppIcon name="menu" />
    </UiButton>
    <AppIcon
      v-if="chat.serverView"
      class="channel-header-icon text-faint"
      name="terminal"
      :size="20"
    />
    <UserAvatar
      v-else-if="chat.active?.kind === 'direct'"
      :member="chat.directPartner(chat.active)"
      size="small"
      presence
    />
    <AppIcon
      v-else
      class="channel-header-icon text-faint"
      name="hash"
      :size="20"
    />
    <h2 class="channel-name text-[15px] font-[650] tracking-[-0.01em] whitespace-nowrap shrink-0">
      <button
        v-if="!chat.serverView && chat.active"
        class="channel-name-button"
        :title="chat.active.kind === 'channel' ? 'Channel settings' : 'Profile'"
        @click="openDetails"
      >
        {{ title }}
      </button>
      <template v-else>{{ title }}</template>
    </h2>
    <p
      v-if="chat.serverView"
      class="channel-topic text-[13px] text-muted-foreground whitespace-nowrap text-ellipsis overflow-hidden min-w-0 pl-2.5 ml-0.5 border-l [border-left-style:solid] border-l-border"
    >
      {{ chat.network?.host }}:{{ chat.network?.port }} · {{ chat.network?.nick }}
    </p>
    <p
      v-else-if="chat.active?.topic"
      class="channel-topic text-[13px] text-muted-foreground whitespace-nowrap text-ellipsis overflow-hidden min-w-0 pl-2.5 ml-0.5 border-l [border-left-style:solid] border-l-border"
      :class="{ 'is-clickable': chat.active.kind === 'channel' }"
      :title="stripFormatting(chat.active.topic)"
      @click="chat.active.kind === 'channel' && openDetails()"
    >
      <MessageContent :text="chat.active.topic" />
    </p>
    <div class="header-actions ml-auto flex items-center gap-0.5 shrink-0">
      <span
        v-if="!chat.serverView && chat.active?.kind === 'channel'"
        class="online-count flex items-center gap-1.5 text-[12px] text-muted-foreground mr-2 whitespace-nowrap"
        ><i class="status-dot inline-block w-[7px] h-[7px] bg-success rounded-full shrink-0" />{{
          onlineCount
        }}
        online</span
      >
      <UiButton
        variant="ghost"
        size="icon-sm"
        v-if="!chat.serverView"
        class="favorite-button"
        :class="{ 'is-favorite': chat.active?.favorite }"
        :aria-label="chat.active?.favorite ? 'Remove from favorites' : 'Add to favorites'"
        :title="chat.active?.favorite ? 'Remove from favorites' : 'Add to favorites'"
        :aria-pressed="chat.active?.favorite"
        @click="chat.active && chat.favorite.mutate(chat.active)"
      >
        <AppIcon
          name="star"
          :size="18"
        />
      </UiButton>
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="header-search"
        :class="{
          'beside-member-search':
            membersVisible && !chat.serverView && chat.active?.kind === 'channel',
        }"
        aria-label="Search messages"
        title="Search (⌘K)"
        @click="actions.search()"
      >
        <AppIcon
          name="search"
          :size="18"
        />
      </UiButton>
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="inbox-button"
        :aria-label="inbox.unread ? `Inbox, ${inbox.unread} unread` : 'Inbox'"
        title="Inbox"
        @click="actions.inbox()"
      >
        <AppIcon
          name="inbox"
          :size="18"
        /><span
          v-if="inbox.unread"
          class="mention-badge bg-destructive text-rail text-[11px] min-w-4.5 h-4.5 py-0 px-[5px] flex items-center justify-center rounded-[9px] font-[700] inbox-badge"
          aria-hidden="true"
          >{{ inbox.unread > 99 ? '99+' : inbox.unread }}</span
        >
      </UiButton>
      <UiButton
        variant="ghost"
        size="icon-sm"
        :aria-label="
          chat.active?.kind === 'channel' && !chat.serverView
            ? `Links in #${chat.active.name}`
            : 'Links'
        "
        title="Links"
        @click="
          actions.links(!chat.serverView && chat.active?.kind === 'channel' ? chat.active.id : '')
        "
      >
        <AppIcon
          name="link"
          :size="18"
        />
      </UiButton>
      <UiButton
        variant="ghost"
        size="icon-sm"
        aria-label="Saved messages"
        title="Saved messages"
        @click="actions.saved()"
      >
        <AppIcon
          name="bookmark"
          :size="18"
        />
      </UiButton>
      <UiButton
        variant="ghost"
        size="icon-sm"
        v-if="!chat.serverView && chat.active?.kind === 'channel'"
        class="member-toggle"
        :aria-label="membersVisible ? 'Hide members' : 'Show members'"
        title="Members"
        :aria-pressed="membersVisible"
        @click="emit('toggleMembers')"
      >
        <AppIcon
          name="users"
          :size="18"
        />
      </UiButton>
    </div>
  </header>
</template>
