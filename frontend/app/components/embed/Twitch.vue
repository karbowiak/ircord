<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Channel, video and clip forms: Iframely plugins/domains/twitch.tv (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'twitch',
  priority: 100,
  matches: [
    {
      hosts: ['twitch.tv', 'www.twitch.tv', 'm.twitch.tv', 'go.twitch.tv'],
      paths: ['/*', '/videos/*', '/*/v/*', '/*/video/*', '/*/clip/*'],
    },
    { hosts: ['clips.twitch.tv'], paths: ['/*'] },
  ],
  layout: { ratio: [16, 9] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed', 'page'],
  frames: [
    { hosts: ['player.twitch.tv'], paths: ['/'] },
    { hosts: ['clips.twitch.tv'], paths: ['/embed'], query: ['clip'] },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
