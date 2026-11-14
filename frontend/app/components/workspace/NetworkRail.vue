<script setup vapor lang="ts">
const emit = defineEmits<{ add: [] }>()
const { t } = useI18n()
const $t = t
const chat = useChatStore()
const navigation = useConversationNavigation()
const preferences = usePreferencesStore()
const notifications = useNotificationStore()
const menus = useMenus()
function selectNetwork(id: string) {
  chat.selectNetwork(id)
  void navigation.current()
}
const unread = (id: string) => {
  const list = chat.data?.conversations.filter((c) => c.networkId === id) ?? []
  return list.some((c) => c.mentionCount)
    ? 'mention'
    : list.some((c) => c.unreadCount && notifications.level(c) !== 'none')
      ? 'unread'
      : ''
}
/** The tab's name with what its dot shows: connection first, then mentions or unread. */
function label(network: { id: string; name: string; status: string }) {
  const activity = unread(network.id)
  return [
    network.name,
    network.status === 'error'
      ? t('workspaceUi.networks.connectionError')
      : network.status === 'connecting'
        ? t('workspaceUi.networks.connecting')
        : network.status === 'disconnected' && t('workspaceUi.networks.disconnected'),
    activity === 'mention'
      ? t('workspaceUi.networks.mentions')
      : activity && t('workspaceUi.networks.unread'),
  ]
    .filter(Boolean)
    .join(', ')
}
// Network destinations remain ordinary navigation buttons, with arrow-key shortcuts.
function navigate(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  const tabs = [
    ...(event.currentTarget as HTMLElement).querySelectorAll<HTMLButtonElement>('.network-tab'),
  ]
  const index = tabs.indexOf(event.target as HTMLButtonElement)
  if (index < 0) return
  event.preventDefault()
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? tabs.length - 1
        : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length
  tabs[next]?.focus()
  tabs[next]?.click()
}
</script>
<template>
  <nav
    class="network-rail flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto pl-2 pr-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [touch-action:pan-x_pinch-zoom]"
    :aria-label="$t('workspaceUi.networks.label')"
    @keydown="navigate"
  >
    <button
      v-for="network in chat.data?.networks"
      :key="network.id"
      type="button"
      class="network-button network-tab relative my-auto flex h-8 max-w-[200px] shrink-0 items-center justify-center gap-2 rounded-full px-3.5 text-[length:var(--ui-small)] font-[560] transition-colors hover:bg-accent hover:text-foreground [@media(pointer:coarse)]:min-h-[40px]"
      :class="
        network.id === chat.networkId
          ? 'is-active [background:linear-gradient(90deg,color-mix(in_srgb,var(--accent)_22%,var(--raised)),color-mix(in_srgb,var(--accent)_8%,var(--raised)))] text-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_38%,var(--border))]'
          : 'bg-secondary/45 text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)]'
      "
      :aria-current="network.id === chat.networkId ? 'page' : undefined"
      :aria-label="label(network)"
      :title="
        $t('workspaceUi.networks.status', {
          name: network.name,
          status:
            network.statusDetail ||
            (network.status === 'connected'
              ? $t('workspaceUi.sidebar.connected')
              : network.status === 'connecting'
                ? $t('workspaceUi.networks.connecting')
                : network.status === 'error'
                  ? $t('workspaceUi.networks.connectionError')
                  : $t('workspaceUi.networks.disconnected')),
        })
      "
      @click="selectNetwork(network.id)"
      @contextmenu="menus.network($event, network)"
    >
      <NuxtImg
        v-if="preferences.preferences.networkIcons[network.id]"
        :src="preferences.preferences.networkIcons[network.id]"
        alt=""
        class="size-4 shrink-0 rounded-sm object-cover"
      />
      <span class="truncate">{{ network.name }}</span>
      <!-- Shape as well as colour: a hollow ring while not connected, a ringed dot for mentions,
           a plain dot for unread messages, and a soft green dot when online. -->
      <i
        class="shrink-0 rounded-full"
        :class="
          network.status !== 'connected'
            ? 'network-offline size-2 border-[1.5px] border-solid border-warning'
            : unread(network.id) === 'mention'
              ? 'network-mention size-1.5 bg-destructive outline-1 outline-offset-[1.5px] outline-solid outline-destructive'
              : unread(network.id)
                ? 'network-unread size-1.5 bg-foreground'
                : 'network-online size-1.5 bg-success'
        "
        aria-hidden="true"
      />
    </button>
    <AppTooltip :text="$t('workspaceUi.networks.add')">
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="add-network my-auto shrink-0 rounded-full bg-secondary/45 text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_55%,transparent)] hover:bg-accent hover:text-foreground"
        :aria-label="$t('workspaceUi.networks.add')"
        @click="emit('add')"
        ><AppIcon
          name="plus"
          :size="17"
      /></UiButton>
    </AppTooltip>
  </nav>
</template>
