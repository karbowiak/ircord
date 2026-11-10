<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Native clips: Iframely plugins/domains/giphy.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'giphy-clips',
  priority: 9,
  matches: [{ hosts: ['giphy.com', 'www.giphy.com'], paths: ['/videos/*', '/clips/*'] }],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['media'],
  providers: ['giphy-clips'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedMediaRenderer :embed="embed" /></template>
