<script setup lang="ts">
import { parseNotes, type NoteBlock, type Release } from '../../shared/utils/changelog'

/**
 * One release: its version, date, an optional status badge and its notes. `level` is the heading
 * level of the version title, so it fits the outline of the view it sits in.
 */
const props = withDefaults(defineProps<{ release: Release; badge?: string; level?: number }>(), {
  level: 3,
})
const blocks = computed((): NoteBlock[] => {
  try {
    return parseNotes(props.release.notes)
  } catch {
    // Never lose a release to a parser bug: show its notes as plain text instead.
    return [{ type: 'paragraph', content: [{ text: props.release.notes }] }]
  }
})
const date = computed(() => {
  const time = Date.parse(props.release.date)
  return Number.isNaN(time)
    ? ''
    : new Date(time).toLocaleDateString(undefined, { dateStyle: 'medium' })
})
const title = computed(() =>
  props.release.name && props.release.name !== props.release.version
    ? props.release.name
    : props.release.version,
)
</script>
<template>
  <article
    class="release-entry grid gap-2 w-full py-3.5 px-4 rounded-md bg-card min-w-0"
    :aria-label="release.version"
  >
    <header class="release-heading flex flex-wrap items-baseline gap-[6px_10px]">
      <component
        :is="`h${level}`"
        class="release-title text-[15px] font-[650]"
        >{{ title }}</component
      >
      <span
        v-if="badge"
        class="release-badge text-[11px] font-[600] py-[1px] px-[7px] rounded-[999px] text-primary-foreground bg-primary"
        >{{ badge }}</span
      >
      <time
        v-if="date"
        :datetime="release.date"
        >{{ date }}</time
      >
    </header>
    <div
      v-if="blocks.length"
      class="release-notes text-[13px] leading-[1.6] text-body wrap-anywhere"
    >
      <NoteBlocks
        :blocks="blocks"
        :depth="Math.min(6, level + 1)"
      />
    </div>
    <SettingsDescription v-else> No notes were written for this release. </SettingsDescription>
  </article>
</template>
