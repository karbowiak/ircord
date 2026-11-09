<script setup lang="ts">
import type { TooltipProviderProps } from 'reka-ui'
import { TooltipProvider } from 'reka-ui'

// Dialogs and collapsing panels restore focus for accessibility. That focus should not reopen
// the label after a pointer click; keyboard focus and deliberate hovering still show it.
const props = withDefaults(defineProps<TooltipProviderProps>(), {
  delayDuration: 500,
  ignoreNonKeyboardFocus: true,
})
</script>

<template>
  <TooltipProvider v-bind="props">
    <slot />
  </TooltipProvider>
</template>
