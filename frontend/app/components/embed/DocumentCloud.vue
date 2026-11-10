<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public document readers: Iframely plugins/domains/documentcloud.org.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'documentcloud',
  priority: 100,
  matches: [
    {
      hosts: ['documentcloud.org', 'www.documentcloud.org', 'embed.documentcloud.org'],
      paths: ['/documents/*', '/documents/*/pages/*'],
    },
  ],
  layout: { ratio: [3, 4] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['embed.documentcloud.org'], paths: ['/documents/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
