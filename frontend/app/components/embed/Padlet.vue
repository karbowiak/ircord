<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely plugins/domains/padlet.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'padlet',
  priority: 100,
  matches: [{ hosts: ['padlet.com'], paths: ['/*/*'] }],
  layout: { height: 608 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['padlet.com'], paths: ['/embed/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
