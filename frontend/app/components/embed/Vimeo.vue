<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'vimeo',
  priority: 100,
  matches: [
    {
      hosts: ['vimeo.com', 'www.vimeo.com'],
      paths: [
        '/*',
        '/channels/*/*',
        '/groups/*/videos/*',
        '/album/*/video/*',
        '/showcase/*/video/*',
      ],
    },
    { hosts: ['player.vimeo.com'], paths: ['/video/*'] },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['player.vimeo.com'], paths: ['/video/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
