<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'

export const embedProvider = {
  id: 'tiktok',
  priority: 100,
  matches: [
    {
      hosts: ['tiktok.com', 'www.tiktok.com', 'm.tiktok.com'],
      paths: ['/*/video/*'],
    },
    { hosts: ['vm.tiktok.com', 'vt.tiktok.com'], paths: ['/*'] },
  ],
  layout: { height: 578 },
  resolver: 'message',
  accepts: ['media', 'widget', 'card'],
  providers: ['tiktok', 'oembed', 'page', 'link'],
  frames: [{ hosts: ['www.tiktok.com'], paths: ['/player/v1/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template>
  <EmbedSocialRenderer
    :embed="embed"
    provider="TikTok"
  />
</template>
