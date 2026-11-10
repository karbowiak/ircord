<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Pen routes: Iframely plugins/domains/codepen.io.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'codepen',
  priority: 100,
  matches: [
    {
      hosts: ['codepen.io'],
      paths: [
        '/*/pen/*',
        '/*/details/*',
        '/*/full/*',
        '/*/embed/*',
        '/*/embed/preview/*',
        '/pen/*',
      ],
    },
  ],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [{ hosts: ['codepen.io'], paths: ['/*/embed/*', '/*/embed/preview/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
