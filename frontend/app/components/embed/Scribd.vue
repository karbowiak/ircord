<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public reader only: Iframely plugins/domains/scribd.com/scribd.com.js (MIT, Itteco 2013–2022).
// The upstream private-access error handler is deliberately excluded.
export const embedProvider = {
  id: 'scribd',
  priority: 100,
  matches: [
    {
      hosts: ['*.scribd.com'],
      paths: [
        '/doc/*/**',
        '/document/*/**',
        '/embeds/*/**',
        '/presentation/*/**',
        '/fullscreen/*/**',
      ],
    },
  ],
  layout: { ratio: [3, 4] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['www.scribd.com'], paths: ['/embeds/*/content'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
