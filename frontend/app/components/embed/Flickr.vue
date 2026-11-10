<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Native public photos: Iframely plugins/domains/flickr.com/flickr.photo.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'flickr',
  priority: 100,
  matches: [
    { hosts: ['flickr.com', 'www.flickr.com'], paths: ['/photos/*/*', '/photos/*/*/**'] },
    { hosts: ['flic.kr'], paths: ['/p/*'] },
  ],
  layout: { height: 400 },
  imageSizing: true,
  resolver: 'message',
  accepts: ['media'],
  providers: ['oembed'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedImageRenderer :embed="embed" /></template>
