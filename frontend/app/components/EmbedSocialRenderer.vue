<script setup vapor lang="ts">
import type { EmbedContext } from '../types/embed'
import { nativePostText } from '../../shared/utils/link-media'

const props = defineProps<{ embed: EmbedContext; provider: string }>()
const post = computed(() => (props.embed.content ? nativePostText(props.embed.content) : undefined))
const heading = computed(() => post.value?.title || post.value?.caption.split('\n')[0])
const singleVideo = computed(
  () => props.embed.assets.length === 1 && props.embed.assets[0]?.kind === 'video',
)
</script>
<template>
  <div class="absolute inset-0 min-h-0 min-w-0">
    <div
      v-if="embed.assets.length"
      class="grid h-full min-h-0 min-w-0 gap-2.5 overflow-hidden rounded-xl border border-solid border-border bg-secondary p-3"
      :style="{
        gridTemplateRows: embed.surface === 'catalog' ? 'minmax(0, 1fr)' : 'auto minmax(0, 1fr)',
      }"
    >
      <div
        v-if="embed.surface !== 'catalog'"
        class="flex min-w-0 items-start gap-2"
      >
        <MediaPostText
          class="flex-1"
          :provider="provider"
          :name="post?.name"
          :handle="post?.handle"
          :title="heading"
          :caption="post?.caption"
          :url="embed.url"
        />
        <MediaLinkActions
          :url="embed.url"
          always-visible
          class="shrink-0"
        />
      </div>
      <div
        class="relative min-h-0 min-w-0"
        :class="{
          '[&_.direct-media]:bg-transparent! [&_.direct-media>video]:bg-transparent [&_.direct-media>video]:w-auto [&_.direct-media>video]:max-w-full [&_.direct-media>video]:mx-auto [&_.direct-media-label]:hidden':
            singleVideo,
        }"
      >
        <EmbedMediaRenderer
          :embed="embed"
          :actions="embed.surface === 'catalog'"
          :plain="singleVideo"
        />
      </div>
    </div>
    <EmbedWidgetRenderer
      v-else-if="embed.widget"
      :embed="embed"
    />
    <EmbedPlaceholder
      v-else
      :embed="embed"
    />
  </div>
</template>
