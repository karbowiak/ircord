<script setup lang="ts">
import type { Conversation } from '../../shared/types/chat'
import { serverViewId } from '../stores/chat'
import { draftCount } from '../../shared/utils/drafts'
import { collapsedItems } from '../../shared/utils/sidebar'
import { isSortableSection } from '../../shared/utils/sidebar-order'
import { channelShortcuts, chordUsers } from '../../shared/utils/shortcuts'
import { isConversationPath } from '../../shared/utils/workspace-routes'
const emit = defineEmits<{ search: []; drafts: []; join: []; close: [] }>()
const props = defineProps<{ docked: boolean }>()
const { t } = useI18n()
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
const sidebarMotion = ref(false)
let sidebarMotionTimer: ReturnType<typeof setTimeout> | undefined
function toggleSidebar() {
  sidebarMotion.value = true
  clearTimeout(sidebarMotionTimer)
  sidebarMotionTimer = setTimeout(() => (sidebarMotion.value = false), 220)
  layout.set('showSidebar', !layout.state.showSidebar)
  nextTick(() =>
    sidebar.value
      ?.querySelector<HTMLButtonElement>(
        layout.state.showSidebar ? '.sidebar-collapse' : '.sidebar-restore',
      )
      ?.focus(),
  )
}
onBeforeUnmount(() => clearTimeout(sidebarMotionTimer))
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
const order = useSidebarOrder()
const name = order.name
const muted = (c: Conversation) => notifications.level(c) === 'none'
const shown = computed(() => new Set(inNetwork.value.map((c) => c.id)))
const favorites = computed(() =>
  order.items(chat.networkId, 'favorites').filter((c) => shown.value.has(c.id)),
)
const channels = computed(() =>
  order.items(chat.networkId, 'channels').filter((c) => shown.value.has(c.id)),
)
const directs = computed(() => inNetwork.value.filter((c) => c.kind === 'direct' && !c.favorite))
/** The jump key (Ctrl or ⌘ with 1 to 9) of each of the first nine channels, while it is free. */
const channelNavigation = useChannelNavigation()
const shortcutKeys = useShortcuts()
const numbers = computed(
  () =>
    new Map(
      channelNavigation.value
        .slice(0, channelShortcuts.length)
        .map((c, index) => [c.id, channelShortcuts[index]!] as const)
        .filter(([, chord]) => !chordUsers(shortcutKeys.bindings.value, chord).length),
    ),
)
const conversationList = ref<HTMLElement | null>(null)
const drag = useSidebarDrag(conversationList, (section, id, target, place) => {
  if (isSortableSection(section)) order.move(chat.networkId, section, id, target, place)
})
const dropClass = (sectionId: string, c: Conversation) => {
  if (drag.dragging.value?.section !== sectionId) return undefined
  if (drag.dragging.value.id === c.id) return 'opacity-50'
  if (drag.target.value?.id !== c.id) return undefined
  return drag.target.value.place === 'before'
    ? 'shadow-[inset_0_2px_0_var(--color-primary)]'
    : 'shadow-[inset_0_-2px_0_var(--color-primary)]'
}
const sections = computed(() =>
  [
    { id: 'favorites', label: t('workspaceUi.sidebar.favorites'), items: favorites.value },
    {
      id: 'channels',
      label: t('workspaceUi.sidebar.channels'),
      items: channels.value,
      action: 'join' as const,
    },
    {
      id: 'directs',
      label: t('workspaceUi.sidebar.directMessages'),
      items: directs.value,
      action: 'search' as const,
    },
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
    :aria-label="$t('workspaceUi.sidebar.conversations')"
  >
    <CollapsedSidebar
      v-if="!layout.state.showSidebar"
      @expand="toggleSidebar"
    />
    <Transition
      name="sidebar-content-slide"
      :css="props.docked && sidebarMotion"
    >
      <div
        v-show="layout.state.showSidebar || !props.docked"
        :inert="!layout.state.showSidebar && props.docked"
        class="sidebar-expanded flex min-h-0 flex-1 flex-col"
      >
        <div
          ref="conversationList"
          class="sidebar-content min-h-0 flex-1 overflow-y-auto px-2 pb-4 pt-3 [touch-action:pan-y_pinch-zoom]"
        >
          <div class="flex items-center gap-1">
            <ConversationLink
              class="min-w-0 flex-1"
              :active="route.path === '/search'"
              :aria-label="$t('workspaceUi.sidebar.searchMessages')"
              @click="emit('search')"
              ><AppIcon
                name="search"
                :size="15"
              /><span class="flex-1 truncate">{{
                $t('workspaceUi.sidebar.search')
              }}</span></ConversationLink
            >
            <UiButton
              variant="ghost"
              size="icon-sm"
              class="sidebar-collapse shrink-0 max-[899px]:hidden"
              :aria-label="$t('workspaceUi.sidebar.hideConversations')"
              :title="$t('workspaceUi.sidebar.hideConversations')"
              @click="toggleSidebar"
              ><AppIcon
                name="sidebarClose"
                :size="17"
            /></UiButton>
            <UiButton
              variant="ghost"
              size="icon-sm"
              class="navigation-close hidden shrink-0 max-[899px]:inline-flex"
              :aria-label="$t('workspaceUi.sidebar.closeConversations')"
              @click="emit('close')"
              ><AppIcon
                name="close"
                :size="18"
            /></UiButton>
          </div>
          <ConversationLink
            :active="route.path === '/inbox'"
            :aria-label="
              inbox.unread
                ? $t('workspaceUi.sidebar.inboxUnread', { count: $n(inbox.unread) })
                : $t('workspaceUi.sidebar.inbox')
            "
            @click="actions.inbox()"
            ><AppIcon
              name="inbox"
              :size="15"
            /><span class="flex-1 truncate">{{ $t('workspaceUi.sidebar.inbox') }}</span
            ><span
              v-if="inbox.unread"
              class="min-w-4 rounded-full bg-selection px-1 text-center text-[10px] font-semibold text-primary"
              >{{ inbox.unread > 99 ? `${$n(99)}+` : $n(inbox.unread) }}</span
            ></ConversationLink
          >
          <ConversationLink
            :active="route.path === '/saved'"
            :aria-label="$t('workspaceUi.sidebar.savedMessages')"
            @click="actions.saved()"
            ><AppIcon
              name="bookmark"
              :size="15"
            /><span class="flex-1 truncate">{{
              $t('workspaceUi.sidebar.saved')
            }}</span></ConversationLink
          >
          <ConversationLink
            class="drafts-link"
            :active="route.path === '/drafts'"
            :aria-label="
              drafts
                ? $t('workspaceUi.sidebar.draftsCount', { count: $n(drafts) })
                : $t('workspaceUi.sidebar.draftsEmpty')
            "
            @click="emit('drafts')"
            ><AppIcon
              name="pencil"
              :size="15"
            /><span class="flex-1 truncate">{{ $t('workspaceUi.sidebar.drafts') }}</span
            ><span
              v-if="drafts"
              class="draft-count ml-auto min-w-4 rounded-full bg-secondary px-1 text-center text-[10px] font-semibold text-muted-foreground"
              aria-hidden="true"
              >{{ $n(drafts) }}</span
            ></ConversationLink
          >
          <ConversationLink
            :active="route.path === '/links'"
            :aria-label="$t('workspaceUi.sidebar.links')"
            @click="actions.links()"
            ><AppIcon
              name="link"
              :size="15"
            /><span class="flex-1 truncate">{{
              $t('workspaceUi.sidebar.links')
            }}</span></ConversationLink
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
              /><span class="flex-1 truncate">{{
                $t('workspaceUi.sidebar.server')
              }}</span></ConversationLink
            >
            <button
              ref="networkMenu"
              class="flex size-7 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground [@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]"
              :aria-label="$t('workspaceUi.sidebar.networkOptions')"
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
              :title="$t('workspaceUi.sidebar.unreadOnlyHelp')"
              @click="layout.set('unreadOnly', !layout.state.unreadOnly)"
            >
              <AppIcon
                name="bell"
                :size="12"
              />{{ $t('workspaceUi.sidebar.unreadOnly') }}
            </button>
            <UiButton
              v-if="unreadHere"
              variant="link"
              size="text"
              class="px-2 py-0.5 text-[length:var(--ui-caption)]"
              @click="markRead.network(chat.networkId)"
              >{{ $t('workspaceUi.sidebar.markAllRead') }}</UiButton
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
                v-if="order.custom(chat.networkId, section.id)"
                :text="$t('workspaceUi.sidebar.sortByName')"
                side="top"
                ><UiButton
                  variant="ghost"
                  size="icon-sm"
                  class="sort-by-name ml-auto size-[22px]"
                  :aria-label="$t('workspaceUi.sidebar.sortSection', { section: section.label })"
                  @click="
                    isSortableSection(section.id) && order.sortByName(chat.networkId, section.id)
                  "
                  ><AppIcon
                    name="sortAz"
                    :size="14" /></UiButton
              ></AppTooltip>
              <AppTooltip
                v-if="section.action"
                :text="
                  section.action === 'join'
                    ? $t('join.title')
                    : $t('workspaceUi.sidebar.messageSomeone')
                "
                side="top"
                ><UiButton
                  variant="ghost"
                  size="icon-sm"
                  class="size-[22px]"
                  :aria-label="
                    section.action === 'join'
                      ? $t('join.title')
                      : $t('workspaceUi.sidebar.messageSomeone')
                  "
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
                :class="dropClass(section.id, c)"
                :data-sort-id="isSortableSection(section.id) ? c.id : undefined"
                :data-sort-section="isSortableSection(section.id) ? section.id : undefined"
                :aria-keyshortcuts="
                  numbers.get(c.id)
                    ? `${shortcutKeys.mac ? 'Meta' : 'Control'}+${numbers.get(c.id)!.key}`
                    : undefined
                "
                :active="isConversationPath(route.path) && c.id === chat.activeId"
                :unread="c.unreadCount > 0 && !muted(c)"
                :direct="c.kind === 'direct'"
                :muted="muted(c)"
                :mentions="c.mentionCount"
                :presence="
                  c.kind === 'direct'
                    ? presenceLabel(chat.directPartner(c), t) || undefined
                    : undefined
                "
                @click="navigation.open(c.id)"
                @contextmenu="menus.conversation($event, c)"
                @pointerdown="isSortableSection(section.id) && drag.down($event, section.id, c.id)"
              >
                <UserAvatar
                  v-if="c.kind === 'direct'"
                  :member="chat.directPartner(c)"
                  size="tiny"
                  presence
                />
                <span
                  v-else-if="numbers.has(c.id)"
                  class="channel-number relative inline-flex shrink-0"
                  :class="
                    isConversationPath(route.path) && c.id === chat.activeId
                      ? 'text-primary'
                      : 'text-faint'
                  "
                  ><AppIcon
                    name="hash"
                    :size="15"
                  /><span
                    class="absolute -right-1 -top-1.5 text-[9px] font-semibold leading-none tabular-nums text-muted-foreground"
                    aria-hidden="true"
                    >{{ numbers.get(c.id)!.key }}</span
                  ></span
                >
                <AppIcon
                  v-else
                  name="hash"
                  :size="15"
                />
                <span class="conversation-name min-w-0 flex-1 truncate">{{ name(c) }}</span>
                <span
                  v-if="c.mentionCount"
                  class="mention-badge flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-bold text-rail"
                  aria-hidden="true"
                  >{{ $n(c.mentionCount) }}</span
                >
                <!-- ConversationLink names these states for screen readers. -->
                <i
                  v-else-if="c.unreadCount && !muted(c)"
                  class="unread-dot mr-0.5 size-1.5 rounded-full bg-foreground"
                  aria-hidden="true"
                />
                <AppIcon
                  v-else-if="muted(c)"
                  name="muted"
                  :size="12"
                  class="text-faint"
                />
              </ConversationLink>
              <p
                v-if="!section.items.length && !section.collapsed && section.id === 'directs'"
                class="sidebar-empty px-2 py-1 text-[length:var(--ui-caption)] text-muted-foreground"
              >
                {{ $t('workspaceUi.sidebar.noConversations') }}
              </p>
            </div>
          </section>
        </div>
        <SidebarConnection />
      </div>
    </Transition>
    <slot />
  </aside>
</template>
