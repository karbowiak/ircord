<script setup vapor lang="ts">
import type { EmbedContext } from '../types/embed'
defineProps<{ embed: EmbedContext; minFrameWidth?: number }>()
</script>
<template>
  <div class="absolute inset-0 min-h-0 min-w-0">
    <LinkWidget
      v-if="embed.widget"
      :key="embed.widgetKey"
      class="h-full w-full"
      :widget="embed.widget"
      :title="embed.title"
      :href="embed.url"
      :expired="embed.expired"
      :frames="embed.selection.provider.frames ?? []"
      :min-frame-width="minFrameWidth"
      fill
    />
    <EmbedPlaceholder
      v-else
      :embed="embed"
    />
  </div>
</template>
