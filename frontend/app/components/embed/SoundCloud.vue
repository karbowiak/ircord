<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Player form and fixtures: Iframely plugins/domains/soundcloud.com (MIT, Itteco 2013–2022).
export const embedProvider = {
  id: 'soundcloud',
  priority: 100,
  matches: [
    {
      hosts: ['soundcloud.com', 'www.soundcloud.com', 'm.soundcloud.com'],
      paths: ['/*/*', '/*/sets/*'],
    },
    { hosts: ['w.soundcloud.com'], paths: ['/player'], query: ['url'] },
  ],
  layout: { height: 166 },
  variants: [
    {
      matches: [
        {
          hosts: ['soundcloud.com', 'www.soundcloud.com', 'm.soundcloud.com'],
          paths: ['/*/sets/*'],
        },
      ],
      layout: { height: 450 },
    },
  ],
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [{ hosts: ['w.soundcloud.com'], paths: ['/player'], query: ['url'] }],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
