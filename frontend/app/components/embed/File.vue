<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
export const embedProvider = {
  id: 'file',
  priority: 10,
  matches: [
    {
      extensions: [
        'pdf',
        'doc',
        'docx',
        'xls',
        'xlsx',
        'ppt',
        'pptx',
        'zip',
        'rar',
        '7z',
        'tar',
        'gz',
        'txt',
        'csv',
        'json',
      ],
    },
  ],
  attachments: ['file'],
  layout: { height: 150 },
  resolver: 'message',
  accepts: ['file', 'card'],
} satisfies EmbedDeclaration
</script>
<script setup lang="ts">
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
