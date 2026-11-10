<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'fallback',
  priority: 0,
  fallback: true,
  layout: { height: 150 },
  resolver: 'message',
  accepts: ['card', 'file'],
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template>
  <div class="absolute inset-0 min-h-0 min-w-0">
    <LinkFileCard
      v-if="embed.file"
      class="h-full"
      :file="embed.file"
      :url="embed.url"
      :expired="embed.expired"
    />
    <EmbedPlaceholder
      v-else
      :embed="embed"
    />
  </div>
</template>
