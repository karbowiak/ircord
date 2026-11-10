<script setup vapor lang="ts">
import type { EmbedContext } from '../../types/embed'
/** `actions: false` when the surrounding card shows copy and open itself, as Reddit's header does. */
withDefaults(defineProps<{ embed: EmbedContext; actions?: boolean; plain?: boolean }>(), {
  actions: true,
})
</script>
<template>
  <div class="absolute inset-0 min-h-0 min-w-0">
    <MediaGallery
      v-if="embed.assets.length"
      class="h-full w-full"
      :assets="embed.assets"
      :nearby="embed.nearby"
      :url="embed.url"
      :label="embed.title"
      :actions="false"
      :plain="plain"
      :fill="embed.surface === 'catalog'"
      @ready="embed.ready"
      @viewing="embed.viewing"
      @unavailable="embed.unavailable"
    />
    <EmbedPlaceholder
      v-else
      :embed="embed"
      :actions="actions"
    />
    <MediaLinkActions
      v-if="embed.assets.length && actions !== false"
      :url="embed.url"
      always-visible
      class="absolute end-2 top-2 z-2"
    />
  </div>
</template>
