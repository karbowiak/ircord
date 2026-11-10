<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Player form: Iframely plugins/domains/wistia.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'wistia',
  priority: 100,
  matches: [
    { hosts: ['*.wistia.com'], paths: ['/medias/*'] },
    { hosts: ['fast.wistia.net', 'fast.wistia.com'], paths: ['/embed/iframe/*'] },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['fast.wistia.net', 'fast.wistia.com'], paths: ['/embed/iframe/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
