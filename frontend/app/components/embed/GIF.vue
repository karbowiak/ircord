<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'gif',
  priority: 20,
  matches: [{ extensions: ['gif'] }],
  attachmentTypes: ['image/gif'],
  layout: { height: 200 },
  imageSizing: true,
  resolver: 'message',
  accepts: ['media'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template>
  <EmbedImageRenderer :embed="embed"
    ><template #footer><slot name="footer" /></template
  ></EmbedImageRenderer>
</template>
