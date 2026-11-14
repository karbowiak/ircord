<script setup lang="ts" vapor>
import { useNow } from '@vueuse/core'
import { stripFormatting } from '../../../shared/utils/irc'
import { memberStatusLabel } from '../../utils/member-status'
defineProps<{ title?: string; membersVisible: boolean; searching: boolean }>()
const emit = defineEmits<{ openNav: []; toggleMembers: []; toggleSearch: [] }>()
const { t, n: $n } = useI18n()
const $t = t
const chat = useChatStore()
const statusNow = useNow({ interval: 1000 })
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
const directStatusMember = computed(() =>
  !chat.serverView && chat.active?.kind === 'direct' && !chat.active.party
    ? chat.directPartner(chat.active)
    : undefined,
)
const directStatus = computed(() =>
  memberStatusLabel(directStatusMember.value, statusNow.value.getTime()),
)
function openDetails() {
  const c = chat.active
  if (!c) return
  if (c.party) actions.party('details', c.id)
  else if (c.kind === 'channel') actions.channelSettings(c.id)
  else actions.profile(chat.directPartner(c)?.id ?? '')
}
</script>
<template>
  <header
    class="chat-header relative flex h-[var(--header-height)] min-w-0 shrink-0 items-center gap-2 px-4 after:pointer-events-none after:absolute after:inset-x-4 after:bottom-0 after:h-px after:[background:linear-gradient(90deg,transparent,color-mix(in_srgb,var(--border)_45%,transparent)_20%,color-mix(in_srgb,var(--border)_45%,transparent)_80%,transparent)] max-[700px]:px-1.5"
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
        class="channel-header-icon mr-1.5 shrink-0 text-faint"
        name="terminal"
        :size="19"
      />
      <AppIcon
        v-else-if="chat.active?.party"
        name="users"
        :size="19"
        class="shrink-0 text-primary"
      />
      <UserAvatar
        v-else-if="chat.active?.kind === 'direct'"
        :member="chat.directPartner(chat.active)"
        class="mr-1.5"
        size="small"
        presence
      />
      <AppIcon
        v-else
        class="channel-header-icon shrink-0 text-primary"
        name="hash"
        :size="19"
      />
      <h1
        class="channel-name min-w-0 text-[length:var(--ui-channel-title)] leading-[1.2] font-[650] tracking-[-0.01em] [transform:translateY(-0.08em)]"
      >
        <button
          v-if="!chat.serverView && chat.active"
          class="channel-name-button flex max-w-full min-w-0 flex-col items-start justify-center text-left"
          :title="
            directStatus ||
            (chat.active.party
              ? $t('workspaceUi.party.details')
              : chat.active.kind === 'channel'
                ? $t('workspaceUi.header.channelSettings')
                : $t('workspaceUi.header.profile'))
          "
          @click="openDetails"
        >
          <span class="max-w-full min-w-0 truncate">{{ title }}</span
          ><MemberStatus
            v-if="directStatus"
            data-direct-status
            :member="directStatusMember"
            :now="statusNow.getTime()"
          /><span
            v-if="partnerPresence"
            class="sr-only"
            >, {{ partnerPresence }}</span
          >
        </button>
        <template v-else>{{ title }}</template>
      </h1>
    </div>
    <p
      v-if="chat.serverView"
      class="channel-topic min-w-0 truncate pl-2 text-[length:var(--ui-channel-topic)] text-muted-foreground max-[1099px]:hidden"
    >
      {{ chat.network?.host }}:{{ chat.network?.port }} · {{ chat.network?.nick }}
    </p>
    <p
      v-else-if="chat.active?.party?.description"
      class="channel-topic min-w-0 truncate pl-2 text-[length:var(--ui-channel-topic)] text-muted-foreground max-[1099px]:hidden"
    >
      {{ chat.active.party.description }}
    </p>
    <p
      v-else-if="chat.active?.topic && !chat.active.party"
      class="channel-topic min-w-0 truncate pl-2 text-[length:var(--ui-channel-topic)] text-muted-foreground max-[1099px]:hidden [&.is-clickable]:cursor-pointer [&.is-clickable]:hover:text-body"
      :class="{ 'is-clickable': chat.active.kind === 'channel' }"
      :title="stripFormatting(chat.active.topic)"
      @click="chat.active.kind === 'channel' && openDetails()"
    >
      <MessageContent :text="chat.active.topic" />
    </p>
    <div class="header-actions ml-auto flex shrink-0 items-center gap-1.5">
      <AppTooltip
        v-if="!chat.serverView && chat.active"
        :text="
          chat.active.kind === 'channel' && !chat.active.party
            ? $t('workspaceUi.header.searchChannel', { channel: chat.active.name })
            : $t('workspaceUi.header.searchConversation')
        "
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="conversation-search-toggle rounded-full bg-secondary/45 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent aria-pressed:[background:linear-gradient(90deg,color-mix(in_srgb,var(--accent)_22%,var(--raised)),color-mix(in_srgb,var(--accent)_8%,var(--raised)))] aria-pressed:text-primary aria-pressed:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_35%,transparent)]"
          :aria-label="$t('workspaceUi.header.searchConversation')"
          :aria-pressed="searching"
          @click="emit('toggleSearch')"
          ><AppIcon
            name="search"
            :size="17" /></UiButton
      ></AppTooltip>
      <AppTooltip
        v-if="!chat.serverView && chat.active?.kind === 'channel' && !chat.active.party"
        :text="$t('workspaceUi.header.members')"
        ><UiButton
          variant="ghost"
          size="icon-sm"
          class="member-toggle w-auto gap-1.5 rounded-full bg-secondary/45 px-2.5 text-[length:var(--ui-caption)] tabular-nums shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent aria-pressed:[background:linear-gradient(90deg,color-mix(in_srgb,var(--accent)_22%,var(--raised)),color-mix(in_srgb,var(--accent)_8%,var(--raised)))] aria-pressed:text-primary aria-pressed:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_35%,transparent)]"
          :aria-label="`${$t('workspaceUi.header.members')}, ${$t('workspaceUi.header.online', { count: $n(onlineCount) })}`"
          :aria-pressed="membersVisible"
          @click="emit('toggleMembers')"
          ><AppIcon
            name="users"
            :size="17"
          /><span class="max-[700px]:hidden"
            ><i class="mr-1 inline-block size-1.5 rounded-full bg-success align-middle" />{{
              $n(onlineCount)
            }}</span
          ></UiButton
        ></AppTooltip
      >
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
          class="conversation-options rounded-full bg-secondary/45 shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent"
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
