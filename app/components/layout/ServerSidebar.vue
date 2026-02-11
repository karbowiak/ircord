<template>
  <div class="server-sidebar">
    <ServerIcon
      v-for="server in appStore.serversList"
      :key="server.id"
      :icon="server.abbreviation"
      :label="server.name"
      :is-active="appStore.activeServerId === server.id"
      @click="onServerClick(server.id)"
      @contextmenu="onServerContextMenu($event, server.id)"
    />
    <div class="spacer" />
    <ServerIcon
      icon="+"
      label="Add Network"
      @click="onAddServer"
    />

    <ConnectServerModal
      :is-open="isConnectModalOpen"
      :editing-server-id="editingServerId"
      @close="closeConnectModal"
    />

    <ContextMenu
      :is-open="contextMenu.open"
      :x="contextMenu.x"
      :y="contextMenu.y"
      :items="contextMenuItems"
      @close="closeContextMenu"
      @select="onContextMenuSelect"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ServerIcon from '~/components/server/ServerIcon.vue'
import ConnectServerModal from '~/components/layout/ConnectServerModal.vue'
import ContextMenu from '~/components/ui/ContextMenu.vue'

const appStore = useAppStore()
const isConnectModalOpen = ref(false)
const editingServerId = ref<string | null>(null)
const contextMenu = reactive({
  open: false,
  x: 0,
  y: 0,
  serverId: '' as string,
})

const contextMenuItems = [
  { id: 'disconnect', label: 'Disconnect from server' },
  { id: 'edit', label: 'Edit server' },
  { id: 'delete', label: 'Delete server', danger: true },
]

function onAddServer() {
  editingServerId.value = null
  isConnectModalOpen.value = true
}

function closeConnectModal() {
  isConnectModalOpen.value = false
  editingServerId.value = null
}

function onServerClick(serverId: string) {
  closeContextMenu()
  appStore.setActiveServer(serverId)
}

function onServerContextMenu(event: MouseEvent, serverId: string) {
  contextMenu.open = true
  contextMenu.x = event.clientX
  contextMenu.y = event.clientY
  contextMenu.serverId = serverId
}

function closeContextMenu() {
  contextMenu.open = false
}

async function onContextMenuSelect(action: string) {
  const serverId = contextMenu.serverId
  if (!serverId) return

  switch (action) {
    case 'disconnect': {
      await appStore.disconnectIrcServer(serverId)
      break
    }
    case 'edit': {
      editingServerId.value = serverId
      isConnectModalOpen.value = true
      break
    }
    case 'delete': {
      await appStore.deleteIrcServer(serverId)
      break
    }
  }
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
