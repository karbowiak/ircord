<script setup vapor lang="ts">
import type { EmbedContext } from '../types/embed'
withDefaults(defineProps<{ embed: EmbedContext; actions?: boolean; error?: unknown }>(), {
  actions: true,
})
</script>
<template>
  <LinkCard
    class="absolute inset-0 h-full! w-full"
    :style="embed.selection.provider.imageSizing ? { maxWidth: '480px' } : undefined"
    :card="error ? { ...embed.card, reason: 'fetch_failed' } : embed.card"
    :url="embed.url"
    :pending="!error && embed.pending"
    :expired="embed.expired"
    :offline="embed.offline"
    :actions="actions"
  />
</template>
