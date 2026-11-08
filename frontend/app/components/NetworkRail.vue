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
    <AppTooltip
      v-for="network in chat.data?.networks"
      :key="network.id"
      :text="network.name"
      side="right"
    >
      <button
        class="network-button relative w-10 h-10 shrink-0 rounded-full bg-secondary text-muted-foreground flex items-center justify-center text-[15px] font-[650] [transition:border-radius_0.15s,_background_0.15s,_color_0.15s] hover:rounded-lg before:absolute before:-left-3 before:w-1 before:h-0 before:rounded-[0_4px_4px_0] before:bg-foreground before:content-[''] before:transition-[height] before:duration-150 hover:before:h-4 [&.is-active]:rounded-lg [&.is-active]:before:h-7 [&.is-disconnected>span]:opacity-50 [@media(pointer:coarse)]:select-none [@media(pointer:coarse)]:[-webkit-touch-callout:none]"
        :class="[
          `tone-${network.color}`,
          network.id !== chat.networkId && 'hover:bg-accent hover:text-foreground',
          {
            'is-active': network.id === chat.networkId,
            'is-disconnected': network.status !== 'connected',
            'has-icon overflow-hidden bg-transparent!':
              preferences.preferences.networkIcons[network.id],
          },
        ]"
        :aria-label="network.name"
        :aria-current="network.id === chat.networkId ? 'page' : undefined"
        @click="chat.selectNetwork(network.id)"
        @contextmenu="menus.network($event, network)"
      >
        <img
          v-if="preferences.preferences.networkIcons[network.id]"
          :src="preferences.preferences.networkIcons[network.id]"
          alt=""
          class="size-full object-cover rounded-[inherit]"
        />
        <span v-else>{{ network.initials }}</span
        ><i
          v-if="unread(network.id) && network.id !== chat.networkId"
          class="rail-unread absolute bottom-[-2px] right-[-2px] w-3 h-3 rounded-full bg-foreground border-3 border-solid border-rail [&.rail-mention]:bg-destructive"
          :class="`rail-${unread(network.id)}`"
        />
      </button>
    </AppTooltip>
    <AppTooltip
      text="Add a network"
      side="right"
    >
      <button
        class="network-button add-network relative w-10 h-10 shrink-0 rounded-full flex items-center justify-center text-[15px] font-[650] [transition:border-radius_0.15s,_background_0.15s,_color_0.15s] bg-transparent text-success shadow-[inset_0_0_0_1px_var(--border)] hover:rounded-lg hover:bg-success hover:text-rail hover:shadow-none"
        aria-label="Add a network"
        @click="emit('add')"
      >
        <AppIcon
          name="plus"
          :size="20"
        />
      </button>
    </AppTooltip>
    <span
      v-if="chat.status !== 'connected'"
      class="rail-connection mt-auto w-[7px] h-[7px] bg-warning rounded-full [&.is-connected]:bg-success [&.is-offline]:bg-destructive [&.is-connecting]:animate-[pulse_1.4s_ease-in-out_infinite] [&.is-reconnecting]:animate-[pulse_1.4s_ease-in-out_infinite]"
      :class="`is-${chat.status}`"
      :title="chat.status === 'offline' ? 'Not connected to IRCord' : 'Connecting to IRCord…'"
    />
  </nav>
</template>
