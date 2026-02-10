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

      <div v-if="mediaPreviewUrls.length" class="previews">
        <template v-for="mediaUrl in mediaPreviewUrls" :key="mediaUrl">
          <video
            v-if="shouldAutoplayMedia(mediaUrl) && isVideoUrl(mediaUrl)"
            :src="proxyMediaUrl(mediaUrl)"
            class="image-preview"
            autoplay
            loop
            muted
            playsinline
            preload="metadata"
          />

          <img
            v-else-if="shouldAutoplayMedia(mediaUrl)"
            :src="mediaUrl"
            class="image-preview"
            alt="Image preview"
            loading="lazy"
            decoding="async"
          >

          <div v-else class="image-preview gif-hover-preview">
            <ControlledGif
              :src="mediaUrl"
              :play-on-hover="true"
              @play="setGifPlaying(mediaUrl, true)"
              @pause="setGifPlaying(mediaUrl, false)"
            />
            <span v-if="!gifPlayingMap[mediaUrl]" class="gif-paused-badge">Paused</span>
          </div>
        </template>
      </div>

      <div v-if="youtubeEmbedUrls.length" class="previews">
        <div
          v-for="embedUrl in youtubeEmbedUrls"
          :key="embedUrl"
          class="youtube-preview"
        >
          <iframe
            :src="embedUrl"
            title="YouTube preview"
            loading="lazy"
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
            allowfullscreen
          />
        </div>
      </div>

      <div v-if="linkPreviewUrls.length" class="previews">
        <a
          v-for="linkUrl in linkPreviewUrls"
          :key="linkUrl"
          :href="linkUrl"
          class="link-preview"
          target="_blank"
          rel="noopener noreferrer"
        >
          <img :src="faviconFor(linkUrl)" alt="" class="favicon">
          <div class="link-meta">
            <div class="link-host">{{ hostFor(linkUrl) }}</div>
            <div class="link-path">{{ displayPathFor(linkUrl) }}</div>
          </div>
        </a>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { User } from '~/types'
import { findEmojiByShortcode } from '~/composables/useEmojiData'
import { isAnimatedMediaUrl, isImageUrl, isVideoUrl, proxyMediaUrl } from '~/composables/useMediaUrl'
import ControlledGif from '~/components/chat/ControlledGif.vue'

interface Props {
  author: User
  content: string
  timestamp: string
}

const props = defineProps<Props>()
const appStore = useAppStore()
const urlRegex = /(https?:\/\/[^\s]+)/g
const gifPlayingMap = reactive<Record<string, boolean>>({})

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

function extractUrls(content: string): string[] {
  return Array.from(new Set(content.match(urlRegex) ?? []))
}

function getYouTubeEmbedUrl(url: string): string | null {
  try {
    const parsed = new URL(url)

    if (parsed.hostname.includes('youtu.be')) {
      const id = parsed.pathname.replace('/', '').trim()
      return id ? `https://www.youtube.com/embed/${id}` : null
    }

    if (parsed.hostname.includes('youtube.com')) {
      const id = parsed.searchParams.get('v')
      if (id) return `https://www.youtube.com/embed/${id}`

      if (parsed.pathname.startsWith('/shorts/')) {
        const shortId = parsed.pathname.split('/shorts/')[1]
        return shortId ? `https://www.youtube.com/embed/${shortId}` : null
      }
    }
  } catch {
    return null
  }

  return null
}

const urls = computed(() => extractUrls(props.content))

const mediaPreviewUrls = computed(() =>
  urls.value.filter((url) => isImageUrl(url) || isVideoUrl(url))
)

const youtubeEmbedUrls = computed(() =>
  urls.value
    .map(getYouTubeEmbedUrl)
    .filter((value): value is string => Boolean(value))
)

const linkPreviewUrls = computed(() =>
  urls.value.filter((url) => !isImageUrl(url) && !isVideoUrl(url) && !getYouTubeEmbedUrl(url))
)

const textSegments = computed<Segment[]>(() => {
  const content = props.content
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

function hostFor(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}

function displayPathFor(url: string): string {
  try {
    const parsed = new URL(url)
    const path = parsed.pathname === '/' ? '' : parsed.pathname
    return `${path}${parsed.search}` || url
  } catch {
    return url
  }
}

function faviconFor(url: string): string {
  return `https://www.google.com/s2/favicons?sz=64&domain_url=${encodeURIComponent(url)}`
}

function shouldAutoplayMedia(url: string): boolean {
  if (!isAnimatedMediaUrl(url)) return true
  return appStore.gifAutoplay
}

function setGifPlaying(url: string, isPlaying: boolean) {
  gifPlayingMap[url] = isPlaying
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

.previews {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.image-preview {
  width: 100%;
  max-width: 420px;
  max-height: 320px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  object-fit: cover;
  background-color: rgba(0, 0, 0, 0.25);
}

.gif-hover-preview {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.gif-paused-badge {
  position: absolute;
  top: 8px;
  right: 8px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(0, 0, 0, 0.45);
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: 10px;
  padding: 3px 8px;
  pointer-events: none;
}

.youtube-preview {
  width: min(100%, 480px);
  aspect-ratio: 16 / 9;
  border-radius: 10px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.youtube-preview iframe {
  width: 100%;
  height: 100%;
  border: 0;
}

.link-preview {
  width: min(100%, 480px);
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.09);
  background-color: rgba(0, 0, 0, 0.16);
  text-decoration: none;
}

.link-preview:hover {
  background-color: rgba(0, 0, 0, 0.22);
}

.favicon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.link-meta {
  min-width: 0;
}

.link-host {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-primary);
}

.link-path {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
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

  .image-preview,
  .youtube-preview,
  .link-preview {
    max-width: 100%;
    width: 100%;
  }
}
</style>
