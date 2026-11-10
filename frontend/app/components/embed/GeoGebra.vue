<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public material players: Iframely plugins/domains/geogebra.org.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'geogebra',
  priority: 100,
  matches: [
    {
      hosts: ['geogebra.org', 'www.geogebra.org'],
      paths: ['/m/*', '/classic/*', '/graphing/*', '/student/*'],
    },
  ],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['www.geogebra.org'], paths: ['/material/iframe/id/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
