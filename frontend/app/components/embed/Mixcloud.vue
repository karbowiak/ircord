<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Classic player geometry: Iframely plugins/domains/mixcloud.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'mixcloud',
  priority: 100,
  matches: [{ hosts: ['mixcloud.com', 'www.mixcloud.com'], paths: ['/*/*'] }],
  layout: { height: 120 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['www.mixcloud.com'], paths: ['/widget/iframe'], query: ['feed'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
