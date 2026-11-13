<script setup lang="ts">
import type { DialogContentEmits, DialogContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { DialogContent, DialogOverlay, DialogPortal, useForwardPropsEmits } from 'reka-ui'

defineOptions({ inheritAttrs: false })

const props = defineProps<
  DialogContentProps & {
    class?: HTMLAttributes['class']
    overlayClass?: HTMLAttributes['class']
  }
>()
const emits = defineEmits<DialogContentEmits>()

const delegatedProps = reactiveOmit(props, 'class', 'overlayClass')

const forwarded = useForwardPropsEmits(delegatedProps, emits)

/** The toast sits outside the dialog; using it must not count as clicking away from the dialog. */
function keepOpenForToast(event: Event) {
  const detail = (event as CustomEvent<{ originalEvent?: Event }>).detail
  const target = (detail?.originalEvent?.target ?? event.target) as Element | null
  if (target?.closest?.('.toast')) event.preventDefault()
}
</script>

<template>
  <!-- Above menus and pickers, as the browser's top layer used to put native dialogs. Toasts
       (z-[85]) sit above dialogs so errors raised inside one are still seen. -->
  <DialogPortal>
    <DialogOverlay
      data-slot="dialog-overlay"
      :class="cn('fixed inset-0 z-[70] bg-[rgb(5_5_10_/_65%)]', props.overlayClass)"
    />
    <DialogContent
      data-slot="dialog-content"
      v-bind="{ ...$attrs, ...forwarded }"
      :class="
        cn(
          'fixed inset-0 z-[70] m-auto h-fit w-[min(calc(460px_*_var(--interface-scale,1)),_calc(100vw_-_24px))] max-h-[calc(100dvh_-_48px)] overflow-auto rounded-lg bg-background p-0 text-foreground shadow-popover outline-none',
          props.class,
        )
      "
      @interact-outside="keepOpenForToast"
    >
      <slot />
    </DialogContent>
  </DialogPortal>
</template>
