<script setup lang="ts">
import type { RadioGroupRootEmits, RadioGroupRootProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { RadioGroupRoot, useForwardPropsEmits } from 'reka-ui'

const props = defineProps<RadioGroupRootProps & { class?: HTMLAttributes['class'] }>()
const emits = defineEmits<RadioGroupRootEmits>()

const delegatedProps = reactiveOmit(props, 'class')

const forwarded = useForwardPropsEmits(delegatedProps, emits)

function update(value: RadioGroupRootProps['modelValue']) {
  if (value !== undefined && value !== props.modelValue) emits('update:modelValue', value)
}

/**
 * Reka moves focus with its roving-focus group. Its deferred radio selection can miss a quick
 * key-up, so complete that selection only if the newly focused radio remains unchecked.
 */
function navigate(event: KeyboardEvent) {
  if (!['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key))
    return
  const root = event.currentTarget as HTMLElement
  const previous = document.activeElement
  if (!(event.target instanceof Element) || !event.target.closest('[role="radio"]')) return
  requestAnimationFrame(() => {
    const focused = document.activeElement
    if (
      focused instanceof HTMLElement &&
      focused !== previous &&
      root.contains(focused) &&
      focused.matches('[role="radio"]:not(:disabled)') &&
      focused.getAttribute('aria-checked') !== 'true'
    )
      focused.click()
  })
}
</script>

<template>
  <RadioGroupRoot
    v-slot="slotProps"
    data-slot="radio-group"
    :class="cn('grid gap-3', props.class)"
    v-bind="{ ...forwarded, 'onUpdate:modelValue': update }"
    @keydown.capture="navigate"
  >
    <slot v-bind="slotProps" />
  </RadioGroupRoot>
</template>
