<template>
  <div
    class="dm-item"
    :class="{ 'active': isActive, 'unread': dm.unread }"
    @click="onClick"
  >
    <div class="status-indicator" :class="dm.recipient.status" />
    <div class="info">
      <div class="name">{{ dm.recipient.username }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { DMChannel } from '~/types'

interface Props {
  dm: DMChannel
  isActive?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  isActive: false
})

const emit = defineEmits<{
  click: []
}>()

function onClick() {
  emit('click')
}
</script>

<style scoped>
.dm-item {
  display: flex;
  align-items: center;
  padding: 4px 8px;
  margin: 1px 8px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-muted);
  transition: background-color 0.1s ease, color 0.1s ease;
}

.dm-item:hover {
  background-color: var(--bg-hover);
  color: var(--text-body);
}

.dm-item.active {
  background-color: var(--bg-active);
  color: var(--text-primary);
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 8px;
  flex-shrink: 0;
  background-color: var(--text-faint);
}

.status-indicator.online {
  background-color: var(--accent-green);
}

.status-indicator.away {
  background-color: var(--accent-orange);
}

.status-indicator.offline {
  background-color: var(--text-faint);
}

.info {
  flex: 1;
  min-width: 0;
}

.name {
  font-family: var(--font-mono);
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dm-item.unread .name {
  color: var(--text-primary);
  font-weight: 500;
}
</style>
