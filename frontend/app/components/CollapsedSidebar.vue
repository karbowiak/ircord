<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { chordLabel, isMac, shortcuts } from '../../shared/utils/shortcuts'
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
const label = (name: string, index: number) =>
  `#${name}${shortcuts.channel[index] ? ` · ${chordLabel(shortcuts.channel[index]!, mac)}` : ''}`
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
        :aria-label="tool.label"
        @click="tool.action"
        ><AppIcon
          :name="tool.icon"
          :size="17" /><i
          v-if="tool.label === 'Inbox' && inbox.unread"
          class="absolute right-1 top-1 size-1.5 rounded-full bg-primary"
          aria-label="Unread"
      /></UiButton>
    </AppTooltip>
    <div
      class="mt-3 flex flex-col items-center gap-1"
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
          :aria-label="label(channel.name, index)"
          :aria-current="
            isConversationPath(route.path) && channel.id === chat.activeId ? 'page' : undefined
          "
          :aria-keyshortcuts="
            shortcuts.channel[index]
              ? `${mac ? 'Meta' : 'Control'}+${shortcuts.channel[index]!.key}`
              : undefined
          "
          @click="navigation.open(channel.id)"
          @contextmenu="menus.conversation($event, channel)"
        >
          {{ index + 1
          }}<i
            v-if="
              channel.mentionCount ||
              (channel.unreadCount && notifications.level(channel) !== 'none')
            "
            class="absolute right-0.5 top-0.5 size-1.5 rounded-full"
            :class="channel.mentionCount ? 'bg-destructive' : 'bg-foreground'"
            :aria-label="
              channel.mentionCount ? `${channel.mentionCount} mentions` : 'Unread messages'
            "
          />
        </button>
      </AppTooltip>
    </div>
    <div
      v-if="directs.length"
      class="mt-2 flex flex-col items-center gap-1 border-t [border-top-style:solid] border-border/50 pt-2"
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
          :aria-label="`Direct message with ${partnerName(direct)}`"
          :aria-current="isActive(direct.id) ? 'page' : undefined"
          @click="navigation.open(direct.id)"
          @contextmenu="menus.conversation($event, direct)"
        >
          <UserAvatar
            :member="chat.directPartner(direct)"
            size="tiny"
            presence
          /><i
            v-if="
              direct.mentionCount || (direct.unreadCount && notifications.level(direct) !== 'none')
            "
            class="absolute right-0.5 top-0.5 size-1.5 rounded-full"
            :class="direct.mentionCount ? 'bg-destructive' : 'bg-foreground'"
            :aria-label="
              direct.mentionCount ? `${direct.mentionCount} mentions` : 'Unread messages'
            "
          />
        </button>
      </AppTooltip>
    </div>
  </nav>
</template>
