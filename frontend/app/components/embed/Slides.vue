<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely plugins/domains/slid.es.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'slides',
  priority: 100,
  matches: [{ hosts: ['slides.com'], paths: ['/*/*', '/*/*/embed'] }],
  layout: { ratio: [48, 35] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['slides.com'], paths: ['/*/*/embed'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
