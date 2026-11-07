<script setup lang="ts">
/**
 * Connection problems: IRCord's own link first, then the open network's IRC connection. A newer
 * IRCord build is announced separately, below them, and only while IRCord is connected.
 */
const chat = useChatStore()
const actions = useWorkspaceActions()
const update = useAppUpdate()
</script>
<template>
  <div
    v-if="chat.status !== 'connected'"
    class="connection-banner"
    role="status"
  >
    <AppIcon
      :name="chat.status === 'offline' ? 'offline' : 'refresh'"
      :size="14"
    /><span>{{
      chat.status === 'offline'
        ? 'You’re offline. Showing what was loaded; drafts are kept.'
        : chat.status === 'connecting'
          ? 'Connecting…'
          : 'Reconnecting…'
    }}</span
    ><button @click="chat.reconnect()">Retry now</button>
  </div>
  <div
    v-else-if="chat.network && chat.network.status !== 'connected'"
    class="connection-banner"
    :class="`is-${chat.network.status}`"
    role="status"
  >
    <AppIcon
      :name="chat.network.status === 'connecting' ? 'refresh' : 'offline'"
      :size="14"
    /><span
      ><strong>{{ chat.network.name }}</strong>
      {{
        chat.network.status === 'connecting'
          ? 'is connecting…'
          : chat.network.status === 'error'
            ? `couldn’t connect: ${chat.network.statusDetail ?? 'unknown error'}`
            : 'is disconnected.'
      }}</span
    >
    <button @click="actions.editNetwork(chat.network.id)">Edit network</button>
    <button @click="chat.connection.mutate(chat.network)">
      {{
        chat.network.status === 'connecting'
          ? 'Cancel connection'
          : chat.network.status === 'error'
            ? 'Retry now'
            : 'Connect'
      }}
    </button>
  </div>
  <div
    v-else-if="!chat.data?.demo && chat.active?.kind === 'channel' && !chat.active.joined"
    class="connection-banner"
    role="status"
  >
    <AppIcon
      name="hash"
      :size="14"
    /><span>You’re not in #{{ chat.active.name }}.</span>
    <button
      :disabled="chat.join.isLoading"
      @click="
        chat.join.mutate({
          networkId: chat.active.networkId,
          name: chat.active.target ?? `#${chat.active.name}`,
        })
      "
    >
      Join channel
    </button>
  </div>
  <div
    v-if="chat.status === 'connected' && update.available"
    class="connection-banner is-update"
    role="status"
  >
    <AppIcon
      name="refresh"
      :size="14"
    /><span>IRCord was updated. Reload to get the new version.</span>
    <button @click="update.reload()">Reload now</button>
    <button @click="update.dismiss()">Later</button>
  </div>
</template>
