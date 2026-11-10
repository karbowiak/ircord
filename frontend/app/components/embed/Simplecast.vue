<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public podcast players: Iframely plugins/domains/simplecast.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'simplecast',
  priority: 100,
  matches: [
    { hosts: ['*.simplecast.com'], paths: ['/episodes/*'] },
    { hosts: ['simplecast.com'], paths: ['/s/*'] },
  ],
  layout: { height: 200 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['player.simplecast.com', 'embed.simplecast.com'], paths: ['/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
