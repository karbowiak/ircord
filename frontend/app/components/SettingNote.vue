<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
/** A boxed note beside settings: context, a caveat or where a value comes from. */
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
        'setting-note flex items-start gap-2.5 rounded-md bg-card px-3 py-2.5 text-[12px] leading-[1.5] text-muted-foreground shadow-[inset_0_0_0_1px_var(--border)]',
        props.class,
      )
    "
    ><slot
  /></component>
</template>
