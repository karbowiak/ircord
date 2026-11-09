<script lang="ts">
import type { EmbedDeclaration } from '../../../shared/types/embed'
import type { EmbedContext } from '../../types/embed'
import { nativePostText } from '../../../shared/utils/link-media'
export const embedProvider = {
  id: 'reddit',
  priority: 100,
  matches: [
    {
      hosts: [
        'reddit.com',
        'www.reddit.com',
        'old.reddit.com',
        'new.reddit.com',
        'np.reddit.com',
        'm.reddit.com',
      ],
      paths: ['/r/*/comments/**', '/comments/**', '/gallery/*'],
    },
    { hosts: ['redd.it'], paths: ['/*'] },
  ],
  layout: { height: 420 },
  resolver: 'message',
  accepts: ['media', 'card'],
  providers: ['reddit', 'oembed', 'page', 'link'],
  hideUrl: true,
} satisfies EmbedDeclaration
</script>
<script setup lang="ts">
const props = defineProps<{ embed: EmbedContext }>()
const post = computed(() => (props.embed.content ? nativePostText(props.embed.content) : undefined))
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
      <!-- Copy and open sit in the card's header, beside the title, not over the media. -->
      <div
        v-if="embed.surface !== 'catalog'"
        class="flex min-w-0 items-start gap-2"
      >
        <MediaPostText
          class="flex-1"
          provider="Reddit"
          :name="post?.name"
          :handle="post?.handle"
          :title="post?.title"
          :caption="post?.caption"
          :url="embed.url"
        />
        <MediaLinkActions
          :url="embed.url"
          always-visible
          class="shrink-0"
        />
      </div>
      <div class="relative min-h-0 min-w-0">
        <EmbedMediaRenderer
          :embed="embed"
          :actions="embed.surface === 'catalog'"
        />
      </div>
    </div>
    <!-- Without media, the placeholder card carries its own copy and open. -->
    <EmbedPlaceholder
      v-else
      :embed="embed"
    />
  </div>
</template>
