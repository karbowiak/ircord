<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public fiddle routes: Iframely plugins/domains/jsfiddle.net.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'jsfiddle',
  priority: 100,
  matches: [
    { hosts: ['jsfiddle.net', 'www.jsfiddle.net'], paths: ['/*', '/*/*', '/*/*/*', '/*/*/*/*'] },
  ],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [
    {
      hosts: ['jsfiddle.net'],
      paths: ['/*/embedded/**', '/*/*/embedded/**', '/*/*/*/embedded/**'],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
