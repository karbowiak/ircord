<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'audio',
  priority: 20,
  matches: [{ extensions: ['mp3', 'm4a', 'ogg', 'wav', 'flac', 'aac', 'opus'] }],
  attachments: ['audio'],
  layout: { height: 88 },
  resolver: 'message',
  accepts: ['media'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedMediaRenderer :embed="embed" /></template>
