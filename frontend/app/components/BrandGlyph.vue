<script lang="ts">
// The hash from IRCord's favicon, drawn in the current text colour for places that need a
// monochrome icon (tab bars, avatars). Its path and stroke are read from public/favicon.svg at
// build time, so the favicon stays the one source of the brand mark. Measured once per page,
// not per icon: bot avatars render it in long lists.
import favicon from '../../public/favicon.svg?raw'
import { brandGlyph } from '../utils/brand'

const glyph = brandGlyph(favicon)
</script>
<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 20 })
</script>
<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    :width="size"
    :height="size"
    :viewBox="glyph.viewBox"
    fill="none"
    stroke="currentColor"
    :stroke-width="glyph.strokeWidth"
    stroke-linecap="round"
    aria-hidden="true"
  >
    <path :d="glyph.path" />
  </svg>
</template>
