<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'

export const embedProvider = {
  id: 'instagram',
  priority: 100,
  matches: [
    {
      hosts: ['instagram.com', 'www.instagram.com'],
      paths: ['/p/*', '/reel/*', '/reels/*', '/tv/*'],
    },
  ],
  layout: { height: 420 },
  variants: [
    {
      matches: [
        {
          hosts: ['instagram.com', 'www.instagram.com'],
          paths: ['/reel/*', '/reels/*', '/tv/*'],
        },
      ],
      layout: { height: 500 },
    },
  ],
  resolver: 'message',
  accepts: ['media', 'widget', 'card'],
  providers: ['instagram', 'oembed', 'page', 'link'],
  frames: [
    {
      hosts: ['www.instagram.com'],
      paths: ['/p/*/embed', '/p/*/embed/captioned', '/reel/*/embed', '/tv/*/embed'],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template>
  <EmbedSocialRenderer
    :embed="embed"
    provider="Instagram"
  />
</template>
