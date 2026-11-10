<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Public presentations: Iframely plugins/domains/prezi.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'prezi',
  priority: 100,
  matches: [{ hosts: ['prezi.com', 'www.prezi.com'], paths: ['/*/*'] }],
  layout: { ratio: [11, 8] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['prezi.com'], paths: ['/embed/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template>
  <EmbedWidgetRenderer
    :embed="embed"
    :min-frame-width="480"
  />
</template>
