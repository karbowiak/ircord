<script setup vapor lang="ts">
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
        'search-empty flex flex-col items-center gap-3 px-4 py-14 text-center text-[length:var(--ui-small)] leading-relaxed text-muted-foreground [&_p]:m-0 [&_p]:max-w-[60ch]',
        props.class,
      )
    "
    ><slot
  /></component>
</template>
