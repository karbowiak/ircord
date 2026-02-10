<template>
  <div class="user-panel">
    <div class="status-dot" :class="appStore.currentUser.status" />
    <div class="info">
      <div class="nickname">{{ appStore.currentUser.username }}</div>
      <div class="status-text">{{ statusLabel }}</div>
    </div>
    <div class="actions">
      <div class="icon" title="Settings" @click="onSettings">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
          <path d="M19.14,12.94c0.04-0.3,0.06-0.61,0.06-0.94c0-0.32-0.02-0.64-0.07-0.94l2.03-1.58c0.18-0.14,0.23-0.41,0.12-0.61 l-1.92-3.32c-0.12-0.22-0.37-0.29-0.59-0.22l-2.39,0.96c-0.5-0.38-1.03-0.7-1.62-0.94L14.4,2.81c-0.04-0.24-0.24-0.41-0.48-0.41 h-3.84c-0.24,0-0.43,0.17-0.47,0.41L9.25,5.35C8.66,5.59,8.12,5.92,7.63,6.29L5.24,5.33c-0.22-0.08-0.47,0-0.59,0.22L2.74,8.87 C2.62,9.08,2.66,9.34,2.86,9.48l2.03,1.58C4.84,11.36,4.8,11.69,4.8,12s0.02,0.64,0.07,0.94l-2.03,1.58 c-0.18,0.14-0.23,0.41-0.12,0.61l1.92,3.32c0.12,0.22,0.37,0.29,0.59,0.22l2.39-0.96c0.5,0.38,1.03,0.7,1.62,0.94l0.36,2.54 c0.05,0.24,0.24,0.41,0.48,0.41h3.84c0.24,0,0.44-0.17,0.47-0.41l0.36-2.54c0.59-0.24,1.13-0.56,1.62-0.94l2.39,0.96 c0.22,0.08,0.47,0,0.59-0.22l1.92-3.32c0.12-0.22,0.07-0.47-0.12-0.61L19.14,12.94z M12,15.6c-1.98,0-3.6-1.62-3.6-3.6 s1.62-3.6,3.6-3.6s3.6,1.62,3.6,3.6S13.98,15.6,12,15.6z"/>
        </svg>
      </div>
    </div>

    <SettingsModal :is-open="isSettingsOpen" @close="isSettingsOpen = false" />
  </div>
</template>

<script setup lang="ts">
import SettingsModal from '~/components/layout/SettingsModal.vue'

const appStore = useAppStore()
const isSettingsOpen = ref(false)

const statusLabel = computed(() => {
  switch (appStore.currentUser.status) {
    case 'online': return 'Online'
    case 'away': return 'Away'
    case 'offline': return 'Offline'
    default: return ''
  }
})

function onSettings() {
  isSettingsOpen.value = true
}
</script>

<style scoped>
.user-panel {
  height: 52px;
  background-color: var(--bg-user-panel);
  display: flex;
  align-items: center;
  padding: 0 8px;
  flex-shrink: 0;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  margin-right: 8px;
  flex-shrink: 0;
  background-color: var(--text-faint);
}

.status-dot.online {
  background-color: var(--accent-green);
}

.status-dot.away {
  background-color: var(--accent-orange);
}

.status-dot.offline {
  background-color: var(--text-faint);
}

.info {
  flex: 1;
  min-width: 0;
}

.nickname {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-text {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.actions {
  display: flex;
  gap: 4px;
}

.icon {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-muted);
  transition: background-color 0.1s ease, color 0.1s ease;
}

.icon:hover {
  background-color: var(--bg-hover);
  color: var(--text-body);
}
</style>
