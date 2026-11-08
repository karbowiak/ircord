<script setup lang="ts">
/**
 * A message in a list of results (search, saved, inbox, links): where and when on the first line,
 * then the message. Attributes and listeners go to the button.
 */
defineProps<{ time?: { iso: string; label: string; title?: string } }>()
</script>
<template>
  <button
    type="button"
    class="search-message flex w-full flex-col gap-0.5 rounded-sm p-2 text-left"
  >
    <span class="search-message-location flex gap-2 text-[12px] font-[600] text-muted-foreground"
      ><slot name="location" /><time
        v-if="time"
        :datetime="time.iso"
        :title="time.title"
        >{{ time.label }}</time
      ></span
    >
    <slot />
  </button>
</template>
