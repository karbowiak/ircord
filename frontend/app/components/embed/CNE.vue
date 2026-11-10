<script lang="ts">
import EmbedWidgetRenderer from '../EmbedWidgetRenderer.vue'
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely cnevids.com.js (MIT, Itteco 2013–2022); see the retained license.
// Only the current anonymous oEmbed player is accepted, with autoplay disabled.
export const embedProvider = {
  id: 'cnevids',
  priority: 100,
  matches: [
    {
      hosts: ['player.cnevids.com', 'player-backend.cnevids.com'],
      paths: ['/embed/*/*', '/iframe/video/*'],
    },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['player-backend.cnevids.com'], paths: ['/iframe/video/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
