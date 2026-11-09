<script setup lang="ts">
import type { LabelProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { Label } from 'reka-ui'

const props = defineProps<LabelProps & { class?: HTMLAttributes['class'] }>()

const delegatedProps = reactiveOmit(props, 'class')
</script>

<template>
  <Label
    data-slot="label"
    v-bind="delegatedProps"
    :class="
      cn(
        'block text-[length:var(--ui-small)] font-[600] text-muted-foreground peer-disabled:cursor-not-allowed peer-disabled:opacity-55',
        props.class,
      )
    "
  >
    <slot />
  </Label>
</template>
