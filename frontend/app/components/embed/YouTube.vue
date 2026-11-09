<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'youtube',
  priority: 100,
  matches: [
    {
      hosts: ['youtube.com', 'www.youtube.com', 'm.youtube.com', 'music.youtube.com'],
      paths: ['/watch'],
      query: ['v'],
    },
    {
      hosts: ['youtube.com', 'www.youtube.com', 'm.youtube.com', 'music.youtube.com'],
      paths: ['/shorts/*', '/embed/*', '/live/*'],
    },
    { hosts: ['youtu.be', 'www.youtu.be'], paths: ['/*'] },
    { hosts: ['youtube-nocookie.com', 'www.youtube-nocookie.com'], paths: ['/embed/*'] },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    {
      hosts: ['www.youtube.com', 'www.youtube-nocookie.com', 'youtube.com', 'youtube-nocookie.com'],
      paths: ['/embed/*'],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
