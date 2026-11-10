<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public photos: Iframely plugins/domains/smugmug.com.js (MIT, Itteco 2013–2022).
// Go requires a final i-ID photo page; galleries and rich results cannot become media.
export const embedProvider = {
  id: 'smugmug',
  priority: 9,
  matches: [{ hosts: ['*.smugmug.com'], paths: ['/**'] }],
  layout: { height: 400 },
  imageSizing: true,
  resolver: 'message',
  accepts: ['media'],
  providers: ['smugmug'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedImageRenderer :embed="embed" /></template>
