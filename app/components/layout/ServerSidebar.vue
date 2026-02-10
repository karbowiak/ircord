<template>
  <div class="server-sidebar">
    <ServerIcon
      v-for="server in appStore.serversList"
      :key="server.id"
      :icon="server.abbreviation"
      :label="server.name"
      :is-active="appStore.activeServerId === server.id"
      @click="appStore.setActiveServer(server.id)"
    />
    <div class="spacer" />
    <ServerIcon
      icon="+"
      label="Add Network"
      @click="onAddServer"
    />
  </div>
</template>

<script setup lang="ts">
import ServerIcon from '~/components/server/ServerIcon.vue'

const appStore = useAppStore()

function onAddServer() {
  console.log('Add network clicked')
}
</script>

<style scoped>
.server-sidebar {
  width: 72px;
  height: 100vh;
  background-color: var(--bg-darkest);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  overflow-y: auto;
}

.spacer {
  flex: 1;
}
</style>
