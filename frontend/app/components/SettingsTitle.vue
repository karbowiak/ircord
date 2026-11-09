<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/** The heading at the top of a settings page. */
const props = withDefaults(defineProps<{ as?: string; class?: HTMLAttributes['class'] }>(), {
  as: 'h3',
  class: undefined,
})
</script>
<template>
  <component
    :is="as"
    :class="
      cn(
        'settings-title mb-4 text-[length:var(--ui-title)] font-[700] tracking-[-0.01em]',
        props.class,
      )
    "
    ><slot
  /></component>
</template>
