<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public editor embeds: Iframely plugins/domains/livecodes.io.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'livecodes',
  priority: 100,
  matches: [{ hosts: ['*.livecodes.io'], paths: ['/'] }],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [{ hosts: ['*.livecodes.io'], paths: ['/'], query: ['embed'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
