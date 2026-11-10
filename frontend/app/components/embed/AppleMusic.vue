<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Album, song and video forms: Iframely plugins/domains/itunes.apple.com/apple.music.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'apple-music',
  priority: 100,
  matches: [
    {
      hosts: [
        'music.apple.com',
        'itunes.apple.com',
        'geo.itunes.apple.com',
        'embed.music.apple.com',
      ],
      paths: ['/*/album/**', '/*/song/**', '/*/playlist/**', '/*/music-video/**'],
    },
  ],
  layout: { height: 450 },
  variants: [
    {
      matches: [
        {
          hosts: [
            'music.apple.com',
            'itunes.apple.com',
            'geo.itunes.apple.com',
            'embed.music.apple.com',
          ],
          paths: ['/*/music-video/**'],
        },
      ],
      layout: { ratio: [16, 9] },
    },
    {
      matches: [
        {
          hosts: [
            'music.apple.com',
            'itunes.apple.com',
            'geo.itunes.apple.com',
            'embed.music.apple.com',
          ],
          paths: ['/*/song/**'],
        },
        {
          hosts: [
            'music.apple.com',
            'itunes.apple.com',
            'geo.itunes.apple.com',
            'embed.music.apple.com',
          ],
          paths: ['/*/album/**'],
          query: ['i'],
        },
      ],
      layout: { height: 175 },
    },
  ],
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [
    {
      hosts: ['embed.music.apple.com'],
      paths: ['/*/album/**', '/*/song/**', '/*/playlist/**', '/*/music-video/**'],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
