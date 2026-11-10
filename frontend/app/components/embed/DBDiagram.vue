<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely plugins/domains/dbdiagram.io.js (MIT, Itteco 2013–2022).
// Backend requires anonymous public-content evidence beyond a generic app shell.
export const embedProvider = {
  id: 'dbdiagram',
  priority: 100,
  matches: [{ hosts: ['dbdiagram.io'], paths: ['/d/*', '/embed/*'] }],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['dbdiagram.io'], paths: ['/embed/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
