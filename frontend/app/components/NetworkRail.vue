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
    class="network-rail"
    aria-label="Networks"
  >
    <button
      v-for="network in chat.data?.networks"
      :key="network.id"
      class="network-button"
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
        class="rail-unread"
        :class="`rail-${unread(network.id)}`"
      />
    </button>
    <button
      class="network-button add-network"
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
      class="rail-connection"
      :class="`is-${chat.status}`"
      :title="chat.status === 'offline' ? 'Not connected to IRCord' : 'Connecting to IRCord…'"
    />
  </nav>
</template>
