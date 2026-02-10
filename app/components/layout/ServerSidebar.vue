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

    <ConnectServerModal
      :is-open="isConnectModalOpen"
      @close="isConnectModalOpen = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ServerIcon from '~/components/server/ServerIcon.vue'
import ConnectServerModal from '~/components/layout/ConnectServerModal.vue'

const appStore = useAppStore()
const isConnectModalOpen = ref(false)

function onAddServer() {
  isConnectModalOpen.value = true
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
