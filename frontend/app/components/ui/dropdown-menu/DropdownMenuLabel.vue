<script setup lang="ts">
import type { DropdownMenuLabelProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { DropdownMenuLabel, useForwardProps } from 'reka-ui'

const props = defineProps<DropdownMenuLabelProps & { class?: HTMLAttributes['class'] }>()

const delegatedProps = reactiveOmit(props, 'class')

const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <DropdownMenuLabel
    data-slot="dropdown-menu-label"
    v-bind="forwardedProps"
    :class="
      cn(
        'overflow-hidden text-ellipsis whitespace-nowrap px-2 pt-1 pb-1.5 text-[length:var(--ui-caption)] font-[650] text-faint',
        props.class,
      )
    "
  >
    <slot />
  </DropdownMenuLabel>
</template>
