<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Keyless shared maps: Iframely plugins/domains/google.com/google.custommap.js
// (MIT, Copyright 2013–2022 Itteco Software Corp.; see the backend license notice).
export const embedProvider = {
  id: 'google-maps',
  priority: 100,
  matches: [
    {
      hosts: ['google.com', 'www.google.com'],
      paths: [
        '/maps/d/viewer',
        '/maps/d/edit',
        '/maps/d/embed',
        '/maps/d/u/0/viewer',
        '/maps/d/u/0/edit',
        '/maps/d/u/0/embed',
      ],
      query: ['mid'],
    },
    { hosts: ['www.google.com', 'google.com'], paths: ['/maps/embed'], query: ['pb'] },
  ],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    { hosts: ['www.google.com'], paths: ['/maps/d/embed'], query: ['mid'] },
    { hosts: ['www.google.com'], paths: ['/maps/embed'], query: ['pb'] },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
