<script lang="ts">
import EmbedWidgetRenderer from '../media/EmbedWidgetRenderer.vue'
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely cartodb.com.js (MIT, Itteco 2013–2022); see the retained license.
// Legacy public maps only; current app maps and cartodb aliases need separate review.
export const embedProvider = {
  id: 'carto',
  priority: 100,
  matches: [
    {
      hosts: ['*.carto.com'],
      paths: [
        '/viz/*/public_map',
        '/viz/*/embed_map',
        '/u/*/viz/*/public_map',
        '/u/*/viz/*/embed_map',
      ],
    },
  ],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['*.carto.com'], paths: ['/viz/*/embed_map', '/u/*/viz/*/embed_map'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
