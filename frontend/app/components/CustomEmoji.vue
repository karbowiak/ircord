<script setup lang="ts">
import type { EmojiItem } from '../../shared/types/media'
/** `icon` draws it as a button glyph (toolbars, menus) rather than inline in text. */
const props = defineProps<{ item: EmojiItem; icon?: boolean }>()
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
    :class="icon ? 'icon-custom-emoji size-5 object-contain align-middle' : 'inline-custom-emoji'"
    :src="appearance.animate ? item.url : item.staticUrl || item.url"
    :alt="`:${item.shortcode}:`"
    :title="`:${item.shortcode}:`"
    decoding="async"
    referrerpolicy="no-referrer"
    @error="failed = true"
  >
</template>
