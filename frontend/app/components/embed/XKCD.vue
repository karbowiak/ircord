<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Comic image source: Iframely plugins/domains/xkcd.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'xkcd',
  priority: 100,
  matches: [{ hosts: ['xkcd.com', 'www.xkcd.com'], paths: ['/*'] }],
  layout: { height: 400 },
  imageSizing: true,
  resolver: 'message',
  accepts: ['media'],
  providers: ['page', 'oembed'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedImageRenderer :embed="embed" /></template>
