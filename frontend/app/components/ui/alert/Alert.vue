<script setup lang="ts">
import type { PrimitiveProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import type { AlertVariants } from '.'
import { Primitive } from 'reka-ui'
import { alertVariants } from '.'

const props = withDefaults(
  defineProps<{
    as?: PrimitiveProps['as']
    class?: HTMLAttributes['class']
    variant?: AlertVariants['variant']
    presentation?: AlertVariants['presentation']
  }>(),
  { as: 'div' },
)
</script>

<template>
  <Primitive
    :as="as"
    data-slot="alert"
    role="alert"
    :class="cn(alertVariants({ variant, presentation }), props.class)"
  >
    <slot />
  </Primitive>
</template>
