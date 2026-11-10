<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Native animated images: Iframely plugins/domains/giphy.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'giphy',
  priority: 100,
  matches: [{ hosts: ['giphy.com', 'www.giphy.com'], paths: ['/gifs/*', '/stickers/*'] }],
  layout: { height: 200 },
  imageSizing: true,
  resolver: 'message',
  accepts: ['media'],
  providers: ['oembed', 'page'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedImageRenderer :embed="embed" /></template>
