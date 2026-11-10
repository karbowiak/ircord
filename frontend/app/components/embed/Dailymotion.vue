<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// URL forms: Iframely plugins/domains/dailymotion.com (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'dailymotion',
  priority: 100,
  matches: [
    { hosts: ['dailymotion.com', 'www.dailymotion.com'], paths: ['/video/*', '/embed/video/*'] },
    { hosts: ['dai.ly'], paths: ['/*'] },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    { hosts: ['www.dailymotion.com'], paths: ['/embed/video/*'] },
    { hosts: ['geo.dailymotion.com'], paths: ['/player/*'], query: ['video'] },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
