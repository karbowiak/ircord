<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/** Centred placeholder for an empty, loading or failed list: an icon, a line and an optional action. */
const props = withDefaults(defineProps<{ as?: string; class?: HTMLAttributes['class'] }>(), {
  as: 'div',
  class: undefined,
})
</script>
<template>
  <component
    :is="as"
    :class="
      cn(
        'search-empty flex flex-col items-center gap-2 px-4 py-12 text-center text-[13px] text-faint',
        props.class,
      )
    "
    ><slot
  /></component>
</template>
