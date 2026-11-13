<script setup vapor lang="ts">
defineProps<{ state: 'nsfw' | 'unknown' }>()
const emit = defineEmits<{ reveal: [] }>()
const { t } = useI18n()
</script>
<template>
  <button
    type="button"
    class="sensitive-media absolute inset-0 grid overflow-hidden rounded-lg bg-secondary place-items-center p-2 text-center"
    @click="emit('reveal')"
  >
    <div
      aria-hidden="true"
      class="absolute inset-0 flex items-center justify-center gap-5 blur-xl opacity-50"
    >
      <span class="h-24 w-28 rounded-full bg-muted-foreground/30" />
      <span class="h-20 w-32 rounded-full bg-primary/30" />
    </div>
    <div class="relative grid justify-items-center gap-1">
      <strong class="rounded-md bg-card px-3 py-1 text-[length:var(--ui-small)] text-foreground">{{
        t(state === 'nsfw' ? 'mediaUi.sensitiveMedia.nsfw' : 'mediaUi.sensitiveMedia.unknown')
      }}</strong>
      <span class="line-clamp-2 text-[length:var(--ui-caption)] text-foreground">{{
        t(state === 'nsfw' ? 'mediaUi.sensitiveMedia.hint' : 'mediaUi.sensitiveMedia.unknownHint')
      }}</span>
    </div>
  </button>
</template>
