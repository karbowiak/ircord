<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
// Iframely plugins/domains/maps.yandex.ru (MIT, Itteco 2013–2022).
// Uses the controlled widget, never the upstream JavaScript map template.
export const embedProvider = {
  id: 'yandex-maps',
  priority: 100,
  matches: [{ hosts: ['yandex.ru', 'yandex.com'], paths: ['/maps/**', '/map-widget/v1/**'] }],
  layout: { ratio: [4, 3] },
  resolver: 'message',
  accepts: ['widget'],
  providers: ['oembed'],
  frames: [
    { hosts: ['yandex.ru', 'yandex.com'], paths: ['/map-widget/v1/', '/map-widget/v1/-/*'] },
    { hosts: ['api-maps.yandex.ru', 'api-maps.yandex.com'], paths: ['/frame/v1/-/*'] },
  ],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup vapor lang="ts">
defineProps<{ embed: EmbedContext }>()
</script>
<template><EmbedWidgetRenderer :embed="embed" /></template>
