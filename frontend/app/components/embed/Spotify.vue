<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'spotify',
  priority: 100,
  matches: [
    {
      hosts: ['open.spotify.com'],
      paths: [
        '/track/*',
        '/episode/*',
        '/album/*',
        '/playlist/*',
        '/show/*',
        '/artist/*',
        '/embed/track/*',
        '/embed/episode/*',
        '/embed/album/*',
        '/embed/playlist/*',
        '/embed/show/*',
        '/embed/artist/*',
        '/*/track/*',
        '/*/episode/*',
        '/*/album/*',
        '/*/playlist/*',
        '/*/show/*',
        '/*/artist/*',
      ],
    },
  ],
  layout: { height: 352 },
  variants: [
    {
      matches: [
        {
          hosts: ['open.spotify.com'],
          paths: [
            '/track/*',
            '/episode/*',
            '/embed/track/*',
            '/embed/episode/*',
            '/*/track/*',
            '/*/episode/*',
          ],
        },
      ],
      layout: { height: 152 },
    },
  ],
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['open.spotify.com'], paths: ['/embed/**'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
