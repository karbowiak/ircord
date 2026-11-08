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
</script>

<template>
  <!-- Above menus, pickers and toasts, as the browser's top layer used to put native dialogs. -->
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
          'fixed inset-0 z-[70] m-auto h-fit w-[min(460px,_calc(100vw_-_24px))] max-h-[calc(100dvh_-_48px)] overflow-auto rounded-lg bg-background p-0 text-foreground shadow-popover outline-none',
          props.class,
        )
      "
    >
      <slot />
    </DialogContent>
  </DialogPortal>
</template>
