<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Provider routes: Iframely lib/plugins/system/oembed/providers.json (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'deezer',
  priority: 100,
  matches: [
    {
      hosts: ['deezer.com', 'www.deezer.com'],
      paths: [
        '/track/*',
        '/album/*',
        '/playlist/*',
        '/artist/*',
        '/*/track/*',
        '/*/album/*',
        '/*/playlist/*',
        '/*/artist/*',
      ],
    },
  ],
  layout: { height: 300 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    {
      hosts: ['widget.deezer.com'],
      paths: [
        '/widget/*/track/*',
        '/widget/*/album/*',
        '/widget/*/playlist/*',
        '/widget/*/artist/*',
      ],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
