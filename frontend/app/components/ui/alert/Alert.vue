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
    /**
     * How screen readers hear the text when it appears or changes. `assertive` interrupts (an
     * error after an action), `polite` waits, and `off` is plain text that a control can still
     * reference with aria-describedby. Inline messages default to `assertive`; panels, which are
     * usually present when the page renders, default to `off`.
     */
    live?: 'assertive' | 'polite' | 'off'
  }>(),
  { as: 'div', variant: undefined, presentation: 'panel', class: undefined, live: undefined },
)
const role = computed(() => {
  const live = props.live ?? (props.presentation === 'inline' ? 'assertive' : 'off')
  return live === 'assertive' ? 'alert' : live === 'polite' ? 'status' : undefined
})
</script>

<template>
  <Primitive
    :as="as"
    data-slot="alert"
    :role="role"
    :class="cn(alertVariants({ variant, presentation }), props.class)"
  >
    <slot />
  </Primitive>
</template>
