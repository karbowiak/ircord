<script lang="ts">
import EmbedWidgetRenderer from '../EmbedWidgetRenderer.vue'
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Reviewed direct iframe from https://docs.widgetic.com/#iframe.
// Catalog/app mapping from Iframely widgetic.com.js remains unsupported.
export const embedProvider = {
  id: 'widgetic',
  priority: 100,
  matches: [{ hosts: ['widgetic.com'], paths: ['/embed/*'] }],
  layout: { ratio: [1, 1] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['widgetic.com'], paths: ['/embed/*'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
