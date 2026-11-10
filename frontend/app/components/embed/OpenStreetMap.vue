<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely plugins/domains/openstreetmap.org.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'openstreetmap',
  priority: 100,
  matches: [
    {
      hosts: ['openstreetmap.org', 'www.openstreetmap.org'],
      paths: ['/', '/node/*', '/export/embed.html'],
    },
  ],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['www.openstreetmap.org'], paths: ['/export/embed.html'], query: ['bbox'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
