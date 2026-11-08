<script setup lang="ts">
const emit = defineEmits<{ add: [] }>()
const chat = useChatStore()
const preferences = usePreferencesStore()
const notifications = useNotificationStore()
const menus = useMenus()
const unread = (id: string) => {
  const list = chat.data?.conversations.filter((c) => c.networkId === id) ?? []
  return list.some((c) => c.mentionCount)
    ? 'mention'
    : list.some((c) => c.unreadCount && notifications.level(c) !== 'none')
      ? 'unread'
      : ''
}
</script>
<template>
  <nav
    class="network-rail flex items-center flex-col gap-2 py-3 px-0 bg-rail z-12 overflow-y-auto [scrollbar-width:none] [touch-action:pan-y_pinch-zoom]"
    aria-label="Networks"
  >
    <button
      v-for="network in chat.data?.networks"
      :key="network.id"
      class="network-button relative w-10 h-10 shrink-0 rounded-full bg-secondary text-muted-foreground flex items-center justify-center text-[15px] font-[650] [transition:border-radius_0.15s,_background_0.15s,_color_0.15s]"
      :class="[
        `tone-${network.color}`,
        {
          'is-active': network.id === chat.networkId,
          'is-disconnected': network.status !== 'connected',
          'has-icon': preferences.preferences.networkIcons[network.id],
        },
      ]"
      :aria-label="network.name"
      :aria-current="network.id === chat.networkId ? 'page' : undefined"
      :title="network.name"
      @click="chat.selectNetwork(network.id)"
      @contextmenu="menus.network($event, network)"
    >
      <img
        v-if="preferences.preferences.networkIcons[network.id]"
        :src="preferences.preferences.networkIcons[network.id]"
        alt=""
      />
      <span v-else>{{ network.initials }}</span
      ><i
        v-if="unread(network.id) && network.id !== chat.networkId"
        class="rail-unread absolute bottom-[-2px] right-[-2px] w-3 h-3 rounded-full bg-foreground border-3 border-solid border-rail"
        :class="`rail-${unread(network.id)}`"
      />
    </button>
    <button
      class="network-button add-network relative w-10 h-10 shrink-0 rounded-full flex items-center justify-center text-[15px] font-[650] [transition:border-radius_0.15s,_background_0.15s,_color_0.15s] bg-transparent text-success shadow-[inset_0_0_0_1px_var(--border)]"
      aria-label="Add a network"
      title="Add a network"
      @click="emit('add')"
    >
      <AppIcon
        name="plus"
        :size="20"
      />
    </button>
    <span
      v-if="chat.status !== 'connected'"
      class="rail-connection w-[7px] h-[7px] bg-warning rounded-full"
      :class="`is-${chat.status}`"
      :title="chat.status === 'offline' ? 'Not connected to IRCord' : 'Connecting to IRCord…'"
    />
  </nav>
</template>
