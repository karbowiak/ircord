<script setup vapor lang="ts">
import type { HTMLAttributes } from 'vue'
/** Quiet help text under a field or control. */
const props = withDefaults(defineProps<{ as?: string; class?: HTMLAttributes['class'] }>(), {
  as: 'p',
  class: undefined,
})
</script>
<template>
  <component
    :is="as"
    :class="cn('field-hint block mt-1.5 text-[12px] leading-[1.5] text-faint', props.class)"
    ><slot
  /></component>
</template>
