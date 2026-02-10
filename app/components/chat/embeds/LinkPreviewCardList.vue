<template>
  <div v-if="cards.length" class="previews" :style="mediaScaleStyle">
    <a
      v-for="card in cards"
      :key="card.url"
      :href="card.url"
      class="link-preview"
      target="_blank"
      rel="noopener noreferrer"
    >
      <div class="link-meta">
        <div class="link-host-row">
          <img :src="faviconFor(card.url)" alt="" class="favicon">
          <div class="link-host">{{ card.siteName || hostFor(card.url) }}</div>
        </div>
        <div v-if="card.title" class="link-title">{{ card.title }}</div>
        <div v-if="card.description" class="link-description">{{ card.description }}</div>
        <div class="link-path">{{ displayPathFor(card.canonicalUrl || card.url) }}</div>
      </div>

      <div v-if="card.previewUrl" class="link-media-wrap">
        <video
          v-if="isVideoUrl(card.previewUrl)"
          :src="proxyMediaUrl(card.previewUrl)"
          class="link-media-preview"
          autoplay
          loop
          muted
          playsinline
          preload="metadata"
        />
        <img
          v-else
          :src="card.previewUrl"
          alt=""
          class="link-media-preview"
          loading="lazy"
          decoding="async"
        >

        <div class="media-brand-overlay">
          <span class="media-brand-label">{{ card.siteName || hostFor(card.url) }}</span>
        </div>
      </div>
    </a>
  </div>
</template>

<script setup lang="ts">
import { isVideoUrl, proxyMediaUrl } from '~/composables/useMediaUrl'
import type { LinkPreviewCard } from '~/composables/useMessageEmbeds'

interface Props {
  cards: LinkPreviewCard[]
  mediaScalePercent: number
}

const props = defineProps<Props>()

const mediaScaleStyle = computed(() => ({
  '--media-scale': String(props.mediaScalePercent / 100),
}))

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
</script>

<style scoped>
.previews {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.link-preview {
  width: min(100%, calc(480px * var(--media-scale, 1)));
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
  padding: 12px;
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

.link-host-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.link-host {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-primary);
}

.link-title {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-body);
  margin-top: 2px;
  line-height: 1.35;
}

.link-description {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 3px;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.link-path {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.link-media-wrap {
  width: 100%;
  position: relative;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.2);
}

.link-media-preview {
  width: 100%;
  height: auto;
  display: block;
  object-fit: contain;
}

.media-brand-overlay {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 44px;
  display: flex;
  align-items: flex-end;
  padding: 8px;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.62), rgba(0, 0, 0, 0));
  pointer-events: none;
}

.media-brand-label {
  font-family: var(--font-mono);
  font-size: 11px;
  color: rgba(255, 255, 255, 0.92);
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 999px;
  padding: 2px 8px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 600px) {
  .link-preview {
    max-width: 100%;
    width: 100%;
  }
}
</style>
