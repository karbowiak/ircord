<script setup lang="ts">
import type { CheckboxRootEmits, CheckboxRootProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { CheckboxIndicator, CheckboxRoot, useForwardPropsEmits } from 'reka-ui'

const props = defineProps<CheckboxRootProps & { class?: HTMLAttributes['class'] }>()
const emits = defineEmits<CheckboxRootEmits>()

const delegatedProps = reactiveOmit(props, 'class')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <CheckboxRoot
    data-slot="checkbox"
    v-bind="forwarded"
    :class="
      cn(
        'grid size-4 shrink-0 place-items-center rounded-[4px] border border-solid border-control bg-field text-primary-foreground transition-[background,border-color] duration-[120ms] data-[state=checked]:border-primary data-[state=checked]:bg-primary disabled:cursor-not-allowed disabled:opacity-45',
        props.class,
      )
    "
  >
    <CheckboxIndicator class="grid place-items-center">
      <AppIcon
        name="check"
        :size="12"
      />
    </CheckboxIndicator>
  </CheckboxRoot>
</template>
