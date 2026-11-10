<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public clips: Iframely plugins/domains/d.pr.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'droplr-video',
  priority: 9,
  matches: [{ hosts: ['*.d.pr'], paths: ['/v/*', '/free/v/*'] }],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['media'],
  providers: ['droplr'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedMediaRenderer :embed="embed" /></template>
