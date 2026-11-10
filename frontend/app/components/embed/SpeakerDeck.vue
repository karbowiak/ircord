<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public deck routes: Iframely plugins/domains/speakerdeck.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'speakerdeck',
  priority: 100,
  matches: [{ hosts: ['speakerdeck.com', 'www.speakerdeck.com'], paths: ['/*/*'] }],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [{ hosts: ['speakerdeck.com'], paths: ['/player/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
