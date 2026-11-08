<script setup lang="ts">
import type { SliderRootEmits, SliderRootProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { SliderRange, SliderRoot, SliderThumb, SliderTrack, useForwardPropsEmits } from 'reka-ui'

const props = defineProps<
  SliderRootProps & {
    class?: HTMLAttributes['class']
    /** The accessible name of each thumb (the element with role="slider"). */
    label?: string
  }
>()
const emits = defineEmits<SliderRootEmits>()

const delegatedProps = reactiveOmit(props, 'class', 'label')

const forwarded = useForwardPropsEmits(delegatedProps, emits)
</script>

<template>
  <SliderRoot
    v-slot="{ modelValue }"
    data-slot="slider"
    v-bind="forwarded"
    :class="
      cn(
        'relative flex w-full touch-none select-none items-center data-disabled:opacity-45',
        props.class,
      )
    "
  >
    <SliderTrack class="relative h-1 w-full grow overflow-hidden rounded-full bg-border">
      <SliderRange class="absolute h-full bg-primary" />
    </SliderTrack>
    <SliderThumb
      v-for="(_, index) in modelValue"
      :key="index"
      :aria-label="label"
      class="block size-4 rounded-full border-2 border-solid border-primary bg-background shadow-control transition-[box-shadow] hover:shadow-[0_0_0_4px_color-mix(in_srgb,var(--accent)_20%,transparent)] focus-visible:shadow-[0_0_0_4px_color-mix(in_srgb,var(--accent)_30%,transparent)] focus-visible:outline-none"
    />
  </SliderRoot>
</template>
