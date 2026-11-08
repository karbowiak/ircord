<script setup lang="ts">
import type { Message } from '../../shared/types/chat'
const props = defineProps<{ items: NonNullable<Message['media']> }>()
const pending = computed(() => props.items.filter((item) => item.status === 'loading'))
const unavailable = computed(() =>
  props.items.filter((item) => item.status === 'failed' || item.status === 'removed'),
)
</script>
<template>
  <div
    v-if="pending.length"
    class="hosted-media-status media-frame is-loading"
    role="status"
    aria-live="polite"
    aria-busy="true"
  >
    <AppIcon
      name="image"
      :size="22"
    />
    <span>{{
      pending.length === 1 ? 'Preparing media…' : `Preparing ${pending.length} media files…`
    }}</span>
  </div>
  <div
    v-for="item in unavailable"
    :key="item.source"
    class="hosted-media-error"
    role="status"
  >
    <AppIcon
      name="image"
      :size="16"
    /><span>{{ item.reason || 'Media is unavailable.' }}</span>
  </div>
</template>
