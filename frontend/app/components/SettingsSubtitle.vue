<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/** A small uppercase heading over a group of settings. */
const props = withDefaults(defineProps<{ as?: string; class?: HTMLAttributes['class'] }>(), {
  as: 'h4',
  class: undefined,
})
</script>
<template>
  <component
    :is="as"
    :class="
      cn(
        'settings-subtitle border-b [border-bottom-style:solid] border-b-border pb-1.5 text-[12px] font-[650] uppercase tracking-[0.05em] text-faint',
        props.class,
      )
    "
    ><slot
  /></component>
</template>
