<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely plugins/domains/quizlet.com.js (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'quizlet',
  priority: 100,
  matches: [{ hosts: ['quizlet.com'], paths: ['/*/*', '/*/*/*', '/*/*/*/embed'] }],
  layout: { height: 500 },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    {
      hosts: ['quizlet.com'],
      paths: [
        '/*/flashcards/embed',
        '/*/match/embed',
        '/*/learn/embed',
        '/*/spell/embed',
        '/*/test/embed',
      ],
    },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
