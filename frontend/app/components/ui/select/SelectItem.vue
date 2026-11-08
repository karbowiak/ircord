<script setup lang="ts">
import type { SelectItemProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { SelectItem, SelectItemIndicator, SelectItemText, useForwardProps } from 'reka-ui'

const props = defineProps<SelectItemProps & { class?: HTMLAttributes['class'] }>()

const delegatedProps = reactiveOmit(props, 'class')

const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <SelectItem
    data-slot="select-item"
    v-bind="forwardedProps"
    :class="
      cn(
        'flex min-h-8 cursor-pointer select-none items-center justify-between gap-2.5 rounded-sm px-2 py-1.5 outline-none wrap-anywhere data-highlighted:bg-accent data-[state=checked]:font-[550] data-disabled:cursor-not-allowed data-disabled:opacity-45',
        props.class,
      )
    "
  >
    <SelectItemText><slot /></SelectItemText>
    <SelectItemIndicator class="shrink-0 text-primary">
      <AppIcon
        name="check"
        :size="14"
      />
    </SelectItemIndicator>
  </SelectItem>
</template>
