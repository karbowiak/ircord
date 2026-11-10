<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// URL forms: Iframely plugins/domains/loom.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'loom',
  priority: 100,
  matches: [{ hosts: ['loom.com', 'www.loom.com'], paths: ['/share/*', '/embed/*'] }],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [{ hosts: ['www.loom.com'], paths: ['/embed/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
