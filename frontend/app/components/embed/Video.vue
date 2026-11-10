<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'video',
  priority: 20,
  matches: [{ extensions: ['mp4', 'webm', 'mov', 'm4v'] }],
  attachments: ['video'],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['media'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedMediaRenderer :embed="embed" /></template>
