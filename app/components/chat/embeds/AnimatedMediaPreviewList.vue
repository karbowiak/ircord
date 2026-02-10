<template>
  <div v-if="mediaUrls.length" class="previews media-previews" :style="mediaScaleStyle">
    <template v-for="mediaUrl in mediaUrls" :key="mediaUrl">
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
</template>

<script setup lang="ts">
import { isAnimatedMediaUrl, isVideoUrl, proxyMediaUrl } from '~/composables/useMediaUrl'
import ControlledGif from '~/components/chat/ControlledGif.vue'

interface Props {
  mediaUrls: string[]
  mediaScalePercent: number
  gifAutoplay: boolean
}

const props = defineProps<Props>()
const gifPlayingMap = reactive<Record<string, boolean>>({})

const mediaScaleStyle = computed(() => ({
  '--media-scale': String(props.mediaScalePercent / 100),
}))

function shouldAutoplayMedia(url: string): boolean {
  if (!isAnimatedMediaUrl(url)) return true
  return props.gifAutoplay
}

function setGifPlaying(url: string, isPlaying: boolean) {
  gifPlayingMap[url] = isPlaying
}
</script>

<style scoped>
.previews {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.image-preview {
  width: min(100%, calc(480px * var(--media-scale, 1)));
  height: auto;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  object-fit: contain;
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

@media (max-width: 600px) {
  .image-preview {
    max-width: 100%;
    width: 100%;
  }
}
</style>
