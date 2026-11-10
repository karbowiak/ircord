<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public podcast players: Iframely plugins/domains/art19.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'art19',
  priority: 100,
  matches: [{ hosts: ['art19.com', 'www.art19.com'], paths: ['/shows/*', '/shows/*/episodes/*'] }],
  layout: { height: 200 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['art19.com'], paths: ['/shows/*/embed', '/shows/*/episodes/*/embed'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
