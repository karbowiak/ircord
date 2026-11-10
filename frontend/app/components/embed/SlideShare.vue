<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public slides: Iframely plugins/domains/slideshare.net.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'slideshare',
  priority: 100,
  matches: [{ hosts: ['slideshare.net', 'www.slideshare.net'], paths: ['/*/*', '/slideshow/**'] }],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [{ hosts: ['www.slideshare.net'], paths: ['/slideshow/embed_code/key/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
