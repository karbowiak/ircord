<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { channelShortcuts, chordLabel, chordUsers, isMac } from '../../shared/utils/shortcuts'
import { isConversationPath } from '../../shared/utils/workspace-routes'
const emit = defineEmits<{ expand: [] }>()
const chat = useChatStore()
const route = useRoute()
const navigation = useConversationNavigation()
const actions = useWorkspaceActions()
const inbox = useInboxStore()
const menus = useMenus()
const notifications = useNotificationStore()
const channels = useChannelNavigation()
const shortcutKeys = useShortcuts()
const mac = isMac(navigator.platform)
const tools = [
  { label: 'Search messages', path: '/search', icon: 'search', action: () => actions.search() },
  { label: 'Inbox', path: '/inbox', icon: 'inbox', action: () => actions.inbox() },
  { label: 'Saved messages', path: '/saved', icon: 'bookmark', action: () => actions.saved() },
  { label: 'Drafts', path: '/drafts', icon: 'pencil', action: () => actions.drafts() },
  { label: 'Links', path: '/links', icon: 'link', action: () => actions.links() },
  {
    label: 'Server',
    path: '/server',
    icon: 'terminal',
    action: () => navigation.open(`server:${chat.networkId}`),
  },
]
/** Direct messages follow the numbered channels, unnumbered, in the sidebar's order. */
const directs = computed(() =>
  (chat.data?.conversations ?? [])
    .filter((c) => c.networkId === chat.networkId && c.kind === 'direct')
    .sort((a, b) => Number(!!b.favorite) - Number(!!a.favorite)),
)
const partnerName = (c: Conversation) => chat.directPartner(c)?.displayName ?? c.name
const isActive = (id: string) => isConversationPath(route.path) && id === chat.activeId
const channelKey = (index: number) => {
  const chord = channelShortcuts[index]
  return chord && !chordUsers(shortcutKeys.bindings.value, chord).length ? chord : undefined
}
const label = (name: string, index: number) =>
  `#${name}${channelKey(index) ? ` · ${chordLabel(channelKey(index)!, mac)}` : ''}`
/** What a row's dot shows, appended to its name; as in NetworkRail, mentions get a ringed dot. */
const activity = (c: Conversation) =>
  c.mentionCount ? 'mention' : c.unreadCount && notifications.level(c) !== 'none' ? 'unread' : ''
const activityText = (c: Conversation) =>
  c.mentionCount
    ? `, ${countLabel(c.mentionCount, 'mention', 'mentions')}`
    : activity(c)
      ? ', unread'
      : ''
const dotClass = (c: Conversation) =>
  activity(c) === 'mention'
    ? 'bg-destructive outline-1 outline-offset-[1.5px] outline-solid outline-destructive'
    : 'bg-foreground'
function directLabel(c: Conversation) {
  const presence = presenceLabel(chat.directPartner(c))
  return `Direct message with ${partnerName(c)}${presence ? `, ${presence}` : ''}${activityText(c)}`
}
</script>
<template>
  <nav
    class="sidebar-compact hidden min-h-0 flex-1 flex-col items-center gap-1 overflow-y-auto px-[4px] py-3 min-[900px]:flex"
    aria-label="Compact conversations"
  >
    <AppTooltip
      text="Show conversations"
      side="right"
      ><UiButton
        variant="ghost"
        size="icon-sm"
        class="sidebar-restore shrink-0"
        aria-label="Show conversations"
        @click="emit('expand')"
        ><AppIcon
          name="sidebarOpen"
          :size="18" /></UiButton
    ></AppTooltip>
    <AppTooltip
      v-for="tool in tools"
      :key="tool.label"
      :text="tool.label === 'Inbox' && inbox.unread ? `Inbox, ${inbox.unread} unread` : tool.label"
      side="right"
    >
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="relative shrink-0"
        :class="{ 'bg-selection text-primary': route.path.startsWith(tool.path) }"
        :aria-current="route.path.startsWith(tool.path) ? 'page' : undefined"
        :aria-label="
          tool.label === 'Inbox' && inbox.unread ? `Inbox, ${inbox.unread} unread` : tool.label
        "
        @click="tool.action"
        ><AppIcon
          :name="tool.icon"
          :size="17" /><i
          v-if="tool.label === 'Inbox' && inbox.unread"
          class="absolute right-1 top-1 size-1.5 rounded-full bg-primary"
          aria-hidden="true"
      /></UiButton>
    </AppTooltip>
    <div
      class="mt-3 flex flex-col items-center gap-1"
      role="group"
      aria-label="Channels"
    >
      <AppTooltip
        v-for="(channel, index) in channels"
        :key="channel.id"
        :text="label(channel.name, index)"
        side="right"
      >
        <button
          type="button"
          class="numbered-channel relative flex size-8 shrink-0 items-center justify-center rounded-md text-[length:var(--ui-text)] font-medium tabular-nums transition-colors"
          :class="
            isConversationPath(route.path) && channel.id === chat.activeId
              ? 'bg-selection text-primary'
              : 'text-muted-foreground hover:bg-accent hover:text-foreground'
          "
          :data-channel-number="index + 1"
          :aria-label="`${label(channel.name, index)}${activityText(channel)}`"
          :aria-current="
            isConversationPath(route.path) && channel.id === chat.activeId ? 'page' : undefined
          "
          :aria-keyshortcuts="
            channelKey(index) ? `${mac ? 'Meta' : 'Control'}+${channelKey(index)!.key}` : undefined
          "
          @click="navigation.open(channel.id)"
          @contextmenu="menus.conversation($event, channel)"
        >
          {{ index + 1
          }}<i
            v-if="activity(channel)"
            class="absolute right-0.5 top-0.5 size-1.5 rounded-full"
            :class="dotClass(channel)"
            aria-hidden="true"
          />
        </button>
      </AppTooltip>
    </div>
    <div
      v-if="directs.length"
      class="mt-2 flex flex-col items-center gap-1 border-t [border-top-style:solid] border-border/50 pt-2"
      role="group"
      aria-label="Direct messages"
    >
      <AppTooltip
        v-for="direct in directs"
        :key="direct.id"
        :text="partnerName(direct)"
        side="right"
      >
        <button
          type="button"
          class="collapsed-direct relative flex size-8 shrink-0 items-center justify-center rounded-md transition-colors [&_.presence]:border-sidebar"
          :class="
            isActive(direct.id)
              ? 'bg-selection [&_.presence]:border-selection'
              : 'hover:bg-accent hover:[&_.presence]:border-accent'
          "
          :aria-label="directLabel(direct)"
          :aria-current="isActive(direct.id) ? 'page' : undefined"
          @click="navigation.open(direct.id)"
          @contextmenu="menus.conversation($event, direct)"
        >
          <UserAvatar
            :member="chat.directPartner(direct)"
            size="tiny"
            presence
          /><i
            v-if="activity(direct)"
            class="absolute right-0.5 top-0.5 size-1.5 rounded-full"
            :class="dotClass(direct)"
            aria-hidden="true"
          />
        </button>
      </AppTooltip>
    </div>
  </nav>
</template>
