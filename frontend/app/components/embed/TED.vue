<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Talk and language paths: Iframely plugins/domains/ted.com/ted.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'ted',
  priority: 100,
  matches: [
    {
      hosts: ['ted.com', 'www.ted.com', 'embed.ted.com'],
      paths: ['/talks/*', '/talks/lang/*/*', '/dubbing/*'],
    },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['embed.ted.com'], paths: ['/talks/*', '/talks/lang/*/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
