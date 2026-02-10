<template>
  <div class="message" :class="{ 'is-action': isAction }">
    <div class="avatar">{{ author.avatar }}</div>
    <div class="content">
      <div class="header">
        <span class="username" :class="nickClass">{{ author.username }}</span>
        <span class="timestamp">{{ timestamp }}</span>
      </div>
      <div class="text">
        <template v-for="(segment, idx) in textSegments" :key="`seg-${idx}`">
          <a
            v-if="segment.type === 'link'"
            :href="segment.value"
            class="inline-link"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ segment.value }}
          </a>
          <img
            v-else-if="segment.type === 'emoji-image'"
            :src="segment.src"
            :alt="segment.label"
            :title="segment.label"
            class="inline-emoji-image"
            loading="lazy"
            decoding="async"
          >
          <span
            v-else-if="segment.type === 'emoji-char'"
            class="inline-emoji-char"
            :title="segment.label"
            :aria-label="segment.label"
          >
            {{ segment.value }}
          </span>
          <span v-else>{{ segment.value }}</span>
        </template>
      </div>

      <AnimatedMediaPreviewList
        :media-urls="mediaPreviewUrls"
        :media-scale-percent="appStore.mediaScalePercent"
        :gif-autoplay="appStore.gifAutoplay"
      />

      <YouTubeEmbedList
        :embed-urls="youtubeEmbedUrls"
        :media-scale-percent="appStore.mediaScalePercent"
      />

      <ResolvingMediaList
        :urls="resolvingPageUrls"
        :media-scale-percent="appStore.mediaScalePercent"
      />

      <LinkPreviewCardList
        :cards="linkPreviewCards"
        :media-scale-percent="appStore.mediaScalePercent"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { User } from '~/types'
import { findEmojiByShortcode } from '~/composables/useEmojiData'
import { useMessageEmbeds } from '~/composables/useMessageEmbeds'
import AnimatedMediaPreviewList from '~/components/chat/embeds/AnimatedMediaPreviewList.vue'
import YouTubeEmbedList from '~/components/chat/embeds/YouTubeEmbedList.vue'
import ResolvingMediaList from '~/components/chat/embeds/ResolvingMediaList.vue'
import LinkPreviewCardList from '~/components/chat/embeds/LinkPreviewCardList.vue'

interface Props {
  author: User
  content: string
  timestamp: string
}

const props = defineProps<Props>()
const appStore = useAppStore()
const { mediaPreviewUrls, youtubeEmbedUrls, resolvingPageUrls, linkPreviewCards } = useMessageEmbeds(toRef(props, 'content'))

const isAction = computed(() => props.content.startsWith('/me '))

const nickClass = computed(() => {
  // Color nicks based on hash of username for variety
  const hash = props.author.username.split('').reduce((acc, char) => {
    return char.charCodeAt(0) + ((acc << 5) - acc)
  }, 0)
  const colors = ['nick-blue', 'nick-green', 'nick-orange', 'nick-purple', 'nick-cyan', 'nick-pink']
  return colors[Math.abs(hash) % colors.length]
})

type Segment = {
  type: 'text' | 'link' | 'emoji-char'
  value: string
  label?: string
} | {
  type: 'emoji-image'
  src: string
  label: string
}

const emojiRegex = /:([a-z0-9_+-]+):/gi

const textSegments = computed<Segment[]>(() => {
  const content = props.content
  const urlRegex = /(https?:\/\/[^\s]+)/g
  const matches = Array.from(content.matchAll(urlRegex))
  if (!matches.length) return applyEmojiShortcodes(content)

  const segments: Segment[] = []
  let cursor = 0

  for (const match of matches) {
    const full = match[0]
    const index = match.index ?? 0

    if (index > cursor) {
      segments.push(...applyEmojiShortcodes(content.slice(cursor, index)))
    }

    segments.push({ type: 'link', value: full })
    cursor = index + full.length
  }

  if (cursor < content.length) {
    segments.push(...applyEmojiShortcodes(content.slice(cursor)))
  }

  return segments
})

function applyEmojiShortcodes(input: string): Segment[] {
  const matches = Array.from(input.matchAll(emojiRegex))
  if (!matches.length) return [{ type: 'text', value: input }]

  const segments: Segment[] = []
  let cursor = 0

  for (const match of matches) {
    const full = match[0]
    const shortcode = match[1]
    const index = match.index ?? 0

    if (index > cursor) {
      segments.push({ type: 'text', value: input.slice(cursor, index) })
    }

    const emoji = findEmojiByShortcode(shortcode)
    if (emoji?.imageUrl) {
      segments.push({
        type: 'emoji-image',
        src: emoji.imageUrl,
        label: `:${emoji.shortcode}:`,
      })
    } else if (emoji?.char) {
      segments.push({
        type: 'emoji-char',
        value: emoji.char,
        label: `:${emoji.shortcode}:`,
      })
    } else {
      segments.push({ type: 'text', value: full })
    }

    cursor = index + full.length
  }

  if (cursor < input.length) {
    segments.push({ type: 'text', value: input.slice(cursor) })
  }

  return segments
}
</script>

<style scoped>
.message {
  position: relative;
  display: flex;
  padding: 2px 16px 2px 72px;
  margin-top: 16px;
  transition: background-color 0.1s ease;
}

.message:first-child {
  margin-top: 0;
}

.message:hover {
  background-color: rgba(0, 0, 0, 0.06);
}

.avatar {
  position: absolute;
  left: 16px;
  top: 2px;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background-color: var(--bg-active);
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-mono);
  font-size: 16px;
  font-weight: 600;
  color: var(--text-body);
  flex-shrink: 0;
}

.content {
  flex: 1;
  min-width: 0;
}

.header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 2px;
}

.username {
  font-family: var(--font-mono);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}

.username:hover {
  text-decoration: underline;
}

/* Nick colors - IRC style */
.nick-blue { color: #7289da; }
.nick-green { color: #43b581; }
.nick-orange { color: #f0a020; }
.nick-purple { color: #b48eff; }
.nick-cyan { color: #00d4aa; }
.nick-pink { color: #f47fff; }

.timestamp {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-faint);
}

.text {
  font-family: var(--font-mono);
  font-size: 14px;
  color: var(--text-body);
  line-height: 1.4;
  word-wrap: break-word;
  white-space: pre-wrap;
}

.inline-link {
  color: #87c7ff;
  text-decoration: none;
}

.inline-link:hover {
  text-decoration: underline;
}

.inline-emoji-char {
  display: inline-block;
  line-height: 1;
  font-size: 18px;
  vertical-align: -3px;
}

.inline-emoji-image {
  width: 20px;
  height: 20px;
  object-fit: contain;
  vertical-align: -4px;
}

.is-action .text {
  color: var(--text-muted);
  font-style: italic;
}

@media (max-width: 600px) {
  .message {
    padding-left: 16px;
  }

  .avatar {
    display: none;
  }

}
</style>
