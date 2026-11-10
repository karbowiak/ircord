<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'gallery',
  priority: 100,
  attachments: ['gallery'],
  layout: { height: 400 },
  resolver: 'none',
  accepts: ['media'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
const props = defineProps<{ embed: EmbedContext }>()
const { t, n } = useI18n()
const label = computed(() => t('mediaUi.gallery.label', { count: n(props.embed.assets.length) }))
</script>
<template>
  <div class="absolute inset-0 min-h-0 min-w-0">
    <MediaGallery
      v-if="embed.assets.length"
      :assets="embed.assets"
      :nearby="embed.nearby"
      :label="label"
      :url="embed.url"
      :actions="false"
      individual-sources
      :fill="embed.surface === 'catalog'"
      @ready="embed.ready"
      @viewing="embed.viewing"
      @unavailable="embed.unavailable"
    />
    <EmbedPlaceholder
      v-else
      :embed="embed"
    />
    <div class="absolute bottom-2 start-2 z-3"><slot name="footer" /></div>
  </div>
</template>
