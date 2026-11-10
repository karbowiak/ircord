<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Compact podcast players: Iframely plugins/domains/libsyn.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'libsyn',
  priority: 100,
  matches: [{ hosts: ['*.libsyn.com', '*.libsynpro.com'], paths: ['/**'] }],
  layout: { height: 96 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    {
      hosts: [
        'html5-player.libsyn.com',
        'player.libsyn.com',
        'play.libsyn.com',
        'html5-play.libsyn.com',
      ],
      paths: ['/embed/episode/id/*/**'],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
