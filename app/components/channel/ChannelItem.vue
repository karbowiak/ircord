<template>
  <div
    class="channel-item"
    :class="{ 'active': isActive, 'unread': channel.unread }"
    @click="onClick"
  >
    <span class="hash">#</span>
    <span class="name">{{ channel.name }}</span>
  </div>
</template>

<script setup lang="ts">
import type { Channel } from '~/types'

interface Props {
  channel: Channel
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
.channel-item {
  display: flex;
  align-items: center;
  padding: 4px 8px;
  margin: 1px 8px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-muted);
  transition: background-color 0.1s ease, color 0.1s ease;
}

.channel-item:hover {
  background-color: var(--bg-hover);
  color: var(--text-body);
}

.channel-item.active {
  background-color: var(--bg-active);
  color: var(--text-primary);
}

.channel-item.unread .name {
  color: var(--text-primary);
  font-weight: 500;
}

.hash {
  font-family: var(--font-mono);
  font-size: 16px;
  margin-right: 6px;
  opacity: 0.5;
}

.name {
  font-family: var(--font-mono);
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
