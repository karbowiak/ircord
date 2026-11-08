<script setup lang="ts">
/**
 * A message in a list of results (search, saved, inbox, links): where and when on the first line,
 * then the message on one line. Attributes and listeners go to the button.
 */
defineProps<{ time?: { iso: string; label: string; title?: string } }>()
</script>
<template>
  <button
    type="button"
    class="search-message flex w-full flex-col gap-0.5 rounded-sm p-2 text-left hover:not-[.is-selected]:bg-accent [&.is-selected]:bg-selection [&_strong]:mr-1.5 [&_strong]:text-foreground [&>span:last-child]:overflow-hidden [&>span:last-child]:text-ellipsis [&>span:last-child]:whitespace-nowrap [&>span:last-child]:text-[14px] [&>span:last-child]:text-body"
  >
    <span class="search-message-location flex gap-2 text-[12px] font-[600] text-muted-foreground"
      ><slot name="location" /><time
        v-if="time"
        class="font-normal text-faint"
        :datetime="time.iso"
        :title="time.title"
        >{{ time.label }}</time
      ></span
    >
    <slot />
  </button>
</template>
