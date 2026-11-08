<script setup lang="ts">
import type { DropdownMenuSubContentEmits, DropdownMenuSubContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { DropdownMenuPortal, DropdownMenuSubContent, useForwardPropsEmits } from 'reka-ui'
import { dropdownMenuContentClass } from '.'

const props = withDefaults(
  defineProps<DropdownMenuSubContentProps & { class?: HTMLAttributes['class'] }>(),
  { sideOffset: 4, collisionPadding: 8 },
)
const emits = defineEmits<DropdownMenuSubContentEmits>()

const delegatedProps = reactiveOmit(props, 'class')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <DropdownMenuPortal>
    <DropdownMenuSubContent
      data-slot="dropdown-menu-sub-content"
      v-bind="forwarded"
      :class="cn(dropdownMenuContentClass, props.class)"
    >
      <slot />
    </DropdownMenuSubContent>
  </DropdownMenuPortal>
</template>
