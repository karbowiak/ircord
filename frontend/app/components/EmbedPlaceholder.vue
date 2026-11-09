<script setup lang="ts">
import type { EmbedContext } from '../types/embed'
defineProps<{ embed: EmbedContext; actions?: boolean }>()
</script>
<template>
  <LinkCard
    class="absolute inset-0 h-full! w-full"
    :card="embed.card"
    :url="embed.url"
    :pending="embed.pending"
    :expired="embed.expired"
    :offline="embed.offline"
    :actions="actions"
  />
</template>
