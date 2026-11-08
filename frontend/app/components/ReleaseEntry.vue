<script setup lang="ts">
import { parseNotes, type Release } from '../../shared/utils/changelog'

/** One release: its version, date, an optional status badge and its notes. */
const props = defineProps<{ release: Release; badge?: string }>()
const blocks = computed(() => parseNotes(props.release.notes))
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
    class="release-entry"
    :aria-label="release.version"
  >
    <header class="release-heading">
      <h3>{{ title }}</h3>
      <span
        v-if="badge"
        class="release-badge"
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
      class="release-notes"
    >
      <NoteBlocks :blocks="blocks" />
    </div>
    <p
      v-else
      class="settings-description"
    >
      No notes were written for this release.
    </p>
  </article>
</template>
