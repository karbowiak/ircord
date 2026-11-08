<script setup lang="ts">
import type { SelectTriggerProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { SelectTrigger, useForwardProps } from 'reka-ui'

const props = defineProps<SelectTriggerProps & { class?: HTMLAttributes['class'] }>()

const delegatedProps = reactiveOmit(props, 'class')

const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <SelectTrigger
    data-slot="select-trigger"
    v-bind="forwardedProps"
    :class="
      cn(
        'group inline-flex min-h-9 w-full items-center justify-between gap-2 rounded-md border border-solid border-border bg-field py-1.5 pr-2 pl-2.5 text-left text-[13px] leading-[1.5] text-foreground outline-none hover:not-disabled:border-[color-mix(in_srgb,var(--muted)_45%,var(--border))] focus-visible:border-[color-mix(in_srgb,var(--accent)_60%,var(--border))] data-[state=open]:border-[color-mix(in_srgb,var(--accent)_60%,var(--border))] aria-invalid:border-destructive disabled:cursor-not-allowed disabled:opacity-55 data-placeholder:text-faint',
        props.class,
      )
    "
  >
    <slot />
  </SelectTrigger>
</template>
