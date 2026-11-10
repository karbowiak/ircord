<script lang="ts">
import EmbedWidgetRenderer from '../media/EmbedWidgetRenderer.vue'
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely issuu.com.js (MIT, Itteco 2013–2022); see the retained license.
// The backend binds both publisher and document; opaque legacy players stay cards.
export const embedProvider = {
  id: 'issuu',
  priority: 100,
  matches: [{ hosts: ['issuu.com'], paths: ['/*/docs/*'] }],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['e.issuu.com'], paths: ['/embed.html'], query: ['u', 'd'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
