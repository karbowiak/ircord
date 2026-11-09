<script setup lang="ts">
import type { RadioGroupItemEmits, RadioGroupItemProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { CircleIcon } from '@lucide/vue'
import { reactiveOmit } from '@vueuse/core'
import { RadioGroupIndicator, RadioGroupItem, useForwardPropsEmits } from 'reka-ui'

const props = withDefaults(
  defineProps<
    RadioGroupItemProps & {
      class?: HTMLAttributes['class']
      variant?: 'default' | 'segment' | 'card' | 'swatch'
    }
  >(),
  { variant: 'default' },
)
const emits = defineEmits<RadioGroupItemEmits>()

const delegatedProps = reactiveOmit(props, 'class', 'variant')

const forwardedProps = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <RadioGroupItem
    data-slot="radio-group-item"
    v-bind="forwardedProps"
    :class="
      cn(
        'disabled:cursor-not-allowed disabled:opacity-45',
        variant === 'default' &&
          'relative aspect-square size-4 shrink-0 rounded-full border border-solid border-input bg-field text-primary aria-invalid:border-destructive',
        variant === 'segment' &&
          'inline-flex min-h-7 items-center gap-1.5 rounded-[calc(var(--radius)-2px)] px-[11px] text-[length:var(--ui-caption)] whitespace-nowrap text-muted-foreground hover:text-foreground data-[state=checked]:bg-secondary data-[state=checked]:text-foreground data-[state=checked]:shadow-control',
        variant === 'card' &&
          'grid gap-1.5 content-start p-2 text-left text-[length:var(--ui-small)] bg-card rounded-md shadow-[inset_0_0_0_1px_var(--border)] transition-[box-shadow,transform] duration-[120ms] hover:shadow-[inset_0_0_0_1px_var(--faint)] data-[state=checked]:shadow-[inset_0_0_0_2px_var(--accent),0_0_0_3px_color-mix(in_srgb,var(--accent)_22%,transparent)]',
        props.class,
      )
    "
  >
    <RadioGroupIndicator
      v-if="variant === 'default'"
      data-slot="radio-group-indicator"
      class="relative flex items-center justify-center"
    >
      <CircleIcon
        class="fill-primary absolute top-1/2 left-1/2 size-2 -translate-x-1/2 -translate-y-1/2"
      />
    </RadioGroupIndicator>
    <slot v-else />
  </RadioGroupItem>
</template>
