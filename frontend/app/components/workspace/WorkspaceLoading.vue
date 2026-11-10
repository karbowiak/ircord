<script setup lang="ts" vapor>
defineProps<{ label: string }>()
</script>
<template>
  <div
    role="status"
    :aria-label="label"
    class="py-2"
  >
    <span class="sr-only">{{ label }}</span>
    <div
      v-for="row in 3"
      :key="row"
      class="flex items-start gap-3 rounded-lg px-4 py-5 animate-pulse motion-reduce:animate-none"
      aria-hidden="true"
    >
      <div class="size-8 shrink-0 rounded-full bg-secondary" />
      <div class="flex flex-1 flex-col gap-3 pt-1">
        <div class="h-2.5 w-1/3 max-w-48 rounded-sm bg-secondary" />
        <div class="h-2.5 w-3/4 max-w-2xl rounded-sm bg-secondary" />
        <div class="h-2.5 w-1/2 max-w-md rounded-sm bg-secondary" />
      </div>
    </div>
  </div>
</template>
