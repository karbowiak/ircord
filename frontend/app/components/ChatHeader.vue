<script setup lang="ts">
import { stripFormatting } from '../../shared/utils/irc'
defineProps<{ title?: string; membersVisible: boolean; searching: boolean }>()
const emit = defineEmits<{ openNav: []; toggleMembers: []; toggleSearch: [] }>()
const { t } = useI18n()
const chat = useChatStore()
const menus = useMenus()
const actions = useWorkspaceActions()
const onlineCount = computed(
  () => chat.activeMemberIds.filter((id) => chat.member(id)?.status === 'online').length,
)
function contextMenu(event: MouseEvent) {
  if (chat.serverView) {
    if (chat.network) menus.network(event, chat.network)
  } else if (chat.active) menus.conversation(event, chat.active)
}
/** The same menu as a right-click, from a button: macOS has no key that opens a context menu. */
function openOptions(event: MouseEvent) {
  const button = event.currentTarget as HTMLElement
  if (chat.serverView) {
    if (chat.network) menus.network(button, chat.network)
  } else if (chat.active) menus.conversation(button, chat.active)
}
/** Presence the DM avatar shows as a dot, for screen readers. */
const partnerPresence = computed(() =>
  chat.active?.kind === 'direct' && !chat.serverView
    ? presenceLabel(chat.directPartner(chat.active), t)
    : '',
)
function openDetails() {
  const c = chat.active
  if (!c) return
  if (c.kind === 'channel') actions.channelSettings(c.id)
  else actions.profile(chat.directPartner(c)?.id ?? '')
}
</script>
<template>
  <header
    class="chat-header flex h-[var(--header-height)] min-w-0 shrink-0 items-center gap-2 px-4 max-[700px]:px-1.5"
    @contextmenu="contextMenu"
  >
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="mobile-menu hidden shrink-0 max-[899px]:inline-flex"
      :aria-label="$t('workspaceUi.sidebar.openConversations')"
      @click="emit('openNav')"
      ><AppIcon
        name="menu"
        :size="18"
    /></UiButton>
    <div class="channel-identity flex min-w-0 max-w-[60%] items-center gap-0.5">
      <AppTooltip
        v-if="!chat.serverView"
        :text="
          chat.active?.favorite
            ? $t('workspaceUi.header.removeFavorite')
            : $t('workspaceUi.header.addFavorite')
        "
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="favorite-button size-6 shrink-0 [&.is-favorite]:text-warning [&.is-favorite_svg]:fill-current max-[700px]:hidden"
          :class="{ 'is-favorite': chat.active?.favorite }"
          :aria-label="
            chat.active?.favorite
              ? $t('workspaceUi.header.removeFavorite')
              : $t('workspaceUi.header.addFavorite')
          "
          :aria-pressed="chat.active?.favorite"
          @click="chat.active && chat.favorite.mutate(chat.active)"
          ><AppIcon
            name="star"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppIcon
        v-if="chat.serverView"
        class="channel-header-icon shrink-0 text-faint"
        name="terminal"
        :size="19"
      />
      <UserAvatar
        v-else-if="chat.active?.kind === 'direct'"
        :member="chat.directPartner(chat.active)"
        size="small"
        presence
      />
      <AppIcon
        v-else
        class="channel-header-icon shrink-0 text-primary"
        name="hash"
        :size="19"
      />
      <h2
        class="channel-name min-w-0 truncate text-[length:var(--ui-channel-title)] leading-[1.2] font-[650] tracking-[-0.01em] [transform:translateY(-0.08em)]"
      >
        <button
          v-if="!chat.serverView && chat.active"
          class="channel-name-button block max-w-full truncate"
          :title="
            chat.active.kind === 'channel'
              ? $t('workspaceUi.header.channelSettings')
              : $t('workspaceUi.header.profile')
          "
          @click="openDetails"
        >
          {{ title
          }}<span
            v-if="partnerPresence"
            class="sr-only"
            >, {{ partnerPresence }}</span
          >
        </button>
        <template v-else>{{ title }}</template>
      </h2>
    </div>
    <p
      v-if="chat.serverView"
      class="channel-topic min-w-0 truncate pl-2 text-[length:var(--ui-channel-topic)] text-muted-foreground max-[1099px]:hidden"
    >
      {{ chat.network?.host }}:{{ chat.network?.port }} · {{ chat.network?.nick }}
    </p>
    <p
      v-else-if="chat.active?.topic"
      class="channel-topic min-w-0 truncate pl-2 text-[length:var(--ui-channel-topic)] text-muted-foreground max-[1099px]:hidden [&.is-clickable]:cursor-pointer [&.is-clickable]:hover:text-body"
      :class="{ 'is-clickable': chat.active.kind === 'channel' }"
      :title="stripFormatting(chat.active.topic)"
      @click="chat.active.kind === 'channel' && openDetails()"
    >
      <MessageContent :text="chat.active.topic" />
    </p>
    <div class="header-actions ml-auto flex shrink-0 items-center gap-0.5">
      <span
        v-if="!chat.serverView && chat.active?.kind === 'channel'"
        class="online-count mr-2 flex items-center gap-1.5 whitespace-nowrap text-[length:var(--ui-caption)] text-muted-foreground max-[1099px]:hidden"
        ><i class="size-1.5 rounded-full bg-success" />{{
          $t('workspaceUi.header.online', { count: $n(onlineCount) })
        }}</span
      >
      <AppTooltip
        v-if="!chat.serverView && chat.active"
        :text="
          chat.active.kind === 'channel'
            ? $t('workspaceUi.header.searchChannel', { channel: chat.active.name })
            : $t('workspaceUi.header.searchConversation')
        "
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="conversation-search-toggle aria-pressed:bg-selection aria-pressed:text-primary"
          :aria-label="$t('workspaceUi.header.searchConversation')"
          :aria-pressed="searching"
          @click="emit('toggleSearch')"
          ><AppIcon
            name="search"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip
        v-if="!chat.serverView && chat.active?.kind === 'channel'"
        :text="$t('workspaceUi.header.members')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="member-toggle aria-pressed:text-foreground"
          :aria-label="$t('workspaceUi.header.members')"
          :aria-pressed="membersVisible"
          @click="emit('toggleMembers')"
          ><AppIcon
            name="users"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip
        v-if="chat.serverView ? chat.network : chat.active"
        :text="
          chat.serverView
            ? $t('workspaceUi.sidebar.networkOptions')
            : $t('workspaceUi.header.conversationOptions')
        "
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="conversation-options"
          :aria-label="
            chat.serverView
              ? $t('workspaceUi.sidebar.networkOptions')
              : $t('workspaceUi.header.conversationOptions')
          "
          aria-haspopup="menu"
          @click="openOptions"
          ><AppIcon
            name="more"
            :size="17" /></UiButton
      ></AppTooltip>
    </div>
  </header>
</template>
