<script setup lang="ts">
import type { Inline } from '../utils/changelog'

/** Changelog text with its bold, code and link parts; never rendered as HTML. */
defineProps<{ content: Inline[] }>()
</script>

<template>
  <template
    v-for="(part, index) in content"
    :key="index"
  >
    <strong
      v-if="part.type === 'strong'"
      class="font-semibold text-ink"
      >{{ part.text }}</strong
    >
    <code
      v-else-if="part.type === 'code'"
      class="rounded-md bg-raised px-1.5 py-0.5 font-mono text-[0.9em]"
      >{{ part.text }}</code
    >
    <a
      v-else-if="part.type === 'link'"
      :href="part.href"
      class="prose-link"
      >{{ part.text }}</a
    >
    <template v-else>{{ part.text }}</template>
  </template>
</template>
