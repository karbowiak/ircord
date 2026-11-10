<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public photos: Iframely plugins/domains/imageshack.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'imageshack',
  priority: 9,
  matches: [{ hosts: ['imageshack.com', 'www.imageshack.com'], paths: ['/i/*'] }],
  layout: { height: 400 },
  imageSizing: true,
  resolver: 'message',
  accepts: ['media'],
  providers: ['imageshack'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedImageRenderer :embed="embed" /></template>
