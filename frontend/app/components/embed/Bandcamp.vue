<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Standard player form: Iframely plugins/domains/bandcamp.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'bandcamp',
  priority: 100,
  matches: [{ hosts: ['*.bandcamp.com'], paths: ['/album/*', '/track/*'] }],
  layout: { height: 120 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [{ hosts: ['bandcamp.com'], paths: ['/EmbeddedPlayer/**'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
