<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public calendar routes: Iframely plugins/domains/google.com/google.calendar.js
// (MIT, Copyright 2013–2022 Itteco Software Corp.; see the backend license notice).
export const embedProvider = {
  id: 'google-calendar',
  priority: 100,
  matches: [
    {
      hosts: ['calendar.google.com', 'www.google.com'],
      paths: [
        '/calendar/embed',
        '/calendar/newembed',
        '/calendar/u/*/embed',
        '/calendar/u/*/newembed',
      ],
      query: ['src'],
    },
  ],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['calendar.google.com'], paths: ['/calendar/embed'], query: ['src'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
