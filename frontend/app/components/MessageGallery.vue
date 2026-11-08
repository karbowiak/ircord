<script setup lang="ts">
import type { ViewerItem } from '../../shared/types/chat'
/**
 * Several images shared together: up to four tiles in a fixed-height grid (so loading never moves
 * the timeline), the fourth marked with how many more there are. Each tile opens the viewer at
 * that image, with the whole set to page through.
 */
const props = defineProps<{
  items: (ViewerItem & { animated?: boolean })[]
  /** Accessible name of the group, such as “4 images from imgur.com”. */
  label: string
}>()
const emit = defineEmits<{ open: [index: number] }>()
const shown = computed(() => props.items.slice(0, 4))
const more = computed(() => props.items.length - shown.value.length)
const failed = reactive(new Set<string>())
</script>
<template>
  <div
    class="message-gallery"
    :class="`has-${shown.length}`"
    role="group"
    :aria-label="label"
  >
    <template
      v-for="(item, index) in shown"
      :key="item.url"
    >
      <a
        v-if="failed.has(item.url)"
        class="gallery-unavailable"
        :href="item.url"
        target="_blank"
        rel="noopener noreferrer"
        >Image unavailable</a
      >
      <MessageImage
        v-else
        class="gallery-tile"
        :src="item.url"
        :label="`View image ${index + 1} of ${items.length}${item.name ? ` · ${item.name}` : ''}`"
        :animated="item.animated"
        external
        @open="emit('open', index)"
        @error="failed.add(item.url)"
      />
    </template>
    <span
      v-if="more > 0"
      class="gallery-more"
      aria-hidden="true"
      >+{{ more }}</span
    >
  </div>
</template>
