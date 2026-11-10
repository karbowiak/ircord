<script setup vapor lang="ts">
import { cn } from '../utils/cn'
import type { HTMLAttributes } from 'vue'
/** A field's label above its control. Attributes such as `for` and `id` go to the label. */
const props = defineProps<{ class?: HTMLAttributes['class'] }>()
</script>
<template>
  <UiLabel
    :class="
      cn('field-label mt-4 mb-1.5 block text-[12px] font-[600] text-muted-foreground', props.class)
    "
    ><slot
  /></UiLabel>
</template>
