<script setup lang="ts">
/**
 * A message in a list of results (search, saved, inbox, links): where and when on the first line,
 * then the message on one line. Attributes and listeners go to the button. `plain` leaves hover
 * and selection to a row around it (saved and inbox rows, which also hold a remove button).
 */
defineProps<{ time?: { iso: string; label: string; title?: string }; plain?: boolean }>()
</script>
<template>
  <button
    type="button"
    class="search-message flex w-full flex-col gap-0.5 rounded-sm p-2 text-left [&_strong]:mr-1.5 [&_strong]:text-foreground [&>span:last-child]:overflow-hidden [&>span:last-child]:text-ellipsis [&>span:last-child]:whitespace-nowrap [&>span:last-child]:text-[14px] [&>span:last-child]:text-body"
    :class="{ 'hover:not-[.is-selected]:bg-accent [&.is-selected]:bg-selection': !plain }"
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
