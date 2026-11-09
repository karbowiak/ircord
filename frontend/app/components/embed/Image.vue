<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'image',
  priority: 20,
  matches: [{ extensions: ['png', 'jpg', 'jpeg', 'webp', 'avif', 'bmp'] }],
  attachments: ['image'],
  layout: { height: 400 },
  resolver: 'message',
  accepts: ['media'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template>
  <EmbedImageRenderer :embed="embed"
    ><template #footer><slot name="footer" /></template
  ></EmbedImageRenderer>
</template>
