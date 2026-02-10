<template>
  <div v-if="isOpen" class="overlay" @click.self="$emit('close')">
    <div class="modal" role="dialog" aria-label="Channel settings">
      <div class="header">
        <h2 class="title">Channel Settings</h2>
        <button type="button" class="close" @click="$emit('close')">x</button>
      </div>

      <div v-if="channel && 'topic' in channel" class="settings-content">
      <div class="row">
        <div class="label">Channel</div>
        <div class="value">#{{ channel.name }}</div>
      </div>

      <div class="row">
        <div class="label">Topic</div>
        <div class="value topic">{{ appStore.getChannelTopic(channel.id) || 'No topic set' }}</div>
      </div>

      <div class="row">
        <div class="label">Modes</div>
        <div class="value">{{ channel.modes || '(none)' }}</div>
      </div>

      <div class="row bans-row">
        <div class="label">Ban List</div>
        <div class="value bans-list">
          <template v-if="settings?.bans?.length">
            <div v-for="ban in settings.bans" :key="`${ban.mask}-${ban.setBy}-${ban.setAt}`" class="ban-entry">
              <span class="mask">{{ ban.mask }}</span>
              <span class="meta">set by {{ ban.setBy || 'unknown' }}{{ ban.setAt ? ` at ${formatBanTime(ban.setAt)}` : '' }}</span>
            </div>
          </template>
          <div v-else class="empty">No bans</div>
        </div>
      </div>

      <div class="actions">
        <button type="button" class="refresh" :disabled="Boolean(settings?.loading)" @click="refresh">
          {{ settings?.loading ? 'Refreshing...' : 'Refresh' }}
        </button>
        <span v-if="settings?.error" class="error">{{ settings.error }}</span>
      </div>
    </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  isOpen: boolean
}>()

defineEmits<{
  (e: 'close'): void
}>()

const appStore = useAppStore()

const channel = computed(() => appStore.activeChannel)
const settings = computed(() => appStore.activeChannelSettings)

async function refresh() {
  const active = appStore.activeChannel
  if (!active || !('topic' in active)) return
  await appStore.refreshChannelSettings(active.id)
}

function formatBanTime(value: string): string {
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}
</script>

<style scoped>
.overlay {
  position: fixed;
  inset: 0;
  z-index: 90;
  background: rgba(0, 0, 0, 0.56);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal {
  width: min(720px, 100%);
  max-height: min(560px, 100%);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(43, 46, 53, 0.98), rgba(37, 40, 47, 0.98));
  box-shadow: 0 26px 50px rgba(0, 0, 0, 0.44);
  padding: 16px;
  overflow-y: auto;
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.title {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 18px;
  color: var(--text-primary);
}

.close {
  border: 0;
  border-radius: 8px;
  padding: 7px 10px;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-muted);
  font-family: var(--font-mono);
  cursor: pointer;
}

.close:hover {
  background: rgba(255, 255, 255, 0.14);
  color: var(--text-primary);
}

.settings-content {
  margin-top: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.row {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 12px;
}

.label {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.value {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-body);
  min-width: 0;
}

.topic {
  word-break: break-word;
}

.bans-row {
  align-items: start;
}

.bans-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 240px;
  overflow-y: auto;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 10px;
  background: rgba(0, 0, 0, 0.16);
}

.ban-entry {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.mask {
  color: var(--text-primary);
}

.meta {
  font-size: 11px;
  color: var(--text-faint);
}

.empty {
  color: var(--text-faint);
}

.actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.refresh {
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.2);
  color: var(--text-body);
  font-family: var(--font-mono);
  font-size: 12px;
  padding: 6px 10px;
  cursor: pointer;
}

.refresh:disabled {
  opacity: 0.7;
  cursor: default;
}

.error {
  font-family: var(--font-mono);
  font-size: 11px;
  color: #ff7d7d;
}

@media (max-width: 680px) {
  .row {
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .label {
    font-size: 11px;
  }
}
</style>
