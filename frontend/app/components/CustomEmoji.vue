<script setup lang="ts">
import type { EmojiItem } from '../../shared/types/media'
const props = defineProps<{ item: EmojiItem }>()
const appearance = useAppearanceStore()
const failed = ref(false)
watch(
  () => props.item.url,
  () => (failed.value = false),
)
</script>
<template>
  <span v-if="failed">:{{ item.shortcode }}:</span
  ><img
    v-else
    class="inline-custom-emoji"
    :src="appearance.animate ? item.url : item.staticUrl || item.url"
    :alt="`:${item.shortcode}:`"
    :title="`:${item.shortcode}:`"
    decoding="async"
    referrerpolicy="no-referrer"
    @error="failed = true"
  />
</template>
