<script setup vapor lang="ts">
import { cn } from '../../utils/cn'
import type { HTMLAttributes } from 'vue'
/** A short paragraph explaining a settings page, dialog or section. */
const props = withDefaults(defineProps<{ as?: string; class?: HTMLAttributes['class'] }>(), {
  as: 'p',
  class: undefined,
})
</script>
<template>
  <component
    :is="as"
    :class="
      cn(
        'settings-description text-[length:var(--ui-small)] leading-[1.55] text-muted-foreground',
        props.class,
      )
    "
    ><slot
  /></component>
</template>
