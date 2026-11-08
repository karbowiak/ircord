<script setup lang="ts">
/**
 * A short label for an icon-only control (the slot), shown on hover and keyboard focus. Touch
 * screens don't show it, so the control still needs its own `aria-label`.
 */
withDefaults(defineProps<{ text: string; side?: 'top' | 'right' | 'bottom' | 'left' }>(), {
  side: 'bottom',
})
</script>
<template>
  <UiTooltip>
    <UiTooltipTrigger as-child><slot /></UiTooltipTrigger>
    <UiTooltipContent :side="side">{{ text }}</UiTooltipContent>
  </UiTooltip>
</template>
