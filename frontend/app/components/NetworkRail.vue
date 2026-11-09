<script setup lang="ts">
const emit = defineEmits<{ add: [] }>()
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
    class="network-rail flex min-w-0 flex-1 items-end gap-1 self-stretch overflow-x-auto px-2 pt-1.5 [scrollbar-width:none] [touch-action:pan-x_pinch-zoom] [@media(pointer:coarse)]:pt-1"
    aria-label="Networks"
    @keydown="navigate"
  >
    <button
      v-for="network in chat.data?.networks"
      :key="network.id"
      type="button"
      class="network-button network-tab relative flex h-9 min-w-24 max-w-44 shrink-0 items-center justify-center gap-2 rounded-t-lg border border-solid px-3 text-[length:var(--ui-text)] font-medium transition-colors hover:bg-secondary hover:text-foreground [@media(pointer:coarse)]:h-[44px]"
      :class="
        network.id === chat.networkId
          ? 'is-active border-border border-b-sidebar bg-sidebar text-foreground before:absolute before:inset-x-3 before:bottom-0 before:h-0.5 before:rounded-full before:bg-primary'
          : 'border-transparent text-muted-foreground'
      "
      :aria-current="network.id === chat.networkId ? 'page' : undefined"
      :aria-label="`${network.name}${unread(network.id) ? ', unread messages' : ''}`"
      :title="`${network.name} · ${network.statusDetail || network.status}`"
      @click="selectNetwork(network.id)"
      @contextmenu="menus.network($event, network)"
    >
      <img
        v-if="preferences.preferences.networkIcons[network.id]"
        :src="preferences.preferences.networkIcons[network.id]"
        alt=""
        class="size-4 shrink-0 rounded-sm object-cover"
      >
      <span class="truncate">{{ network.name }}</span>
      <i
        v-if="network.status !== 'connected' || unread(network.id)"
        class="size-1.5 shrink-0 rounded-full"
        :class="
          network.status !== 'connected'
            ? 'bg-warning'
            : unread(network.id) === 'mention'
              ? 'bg-destructive'
              : 'bg-foreground'
        "
        aria-hidden="true"
      />
    </button>
    <AppTooltip text="Add a network">
      <UiButton
        variant="ghost"
        size="icon-sm"
        class="add-network mb-0.5 shrink-0"
        aria-label="Add a network"
        @click="emit('add')"
        ><AppIcon
          name="plus"
          :size="17"
      /></UiButton>
    </AppTooltip>
  </nav>
</template>
