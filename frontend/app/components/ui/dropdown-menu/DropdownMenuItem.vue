<script setup lang="ts">
import type { DropdownMenuItemEmits, DropdownMenuItemProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import type { DropdownMenuItemVariants } from '.'
import { reactiveOmit } from '@vueuse/core'
import { DropdownMenuItem, useForwardPropsEmits } from 'reka-ui'
import { dropdownMenuItemVariants } from '.'

const props = defineProps<
  DropdownMenuItemProps & {
    class?: HTMLAttributes['class']
    variant?: DropdownMenuItemVariants['variant']
  }
>()
const emits = defineEmits<DropdownMenuItemEmits>()

const delegatedProps = reactiveOmit(props, 'class', 'variant')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <DropdownMenuItem
    data-slot="dropdown-menu-item"
    v-bind="forwarded"
    :class="cn(dropdownMenuItemVariants({ variant }), props.class)"
  >
    <slot />
  </DropdownMenuItem>
</template>
