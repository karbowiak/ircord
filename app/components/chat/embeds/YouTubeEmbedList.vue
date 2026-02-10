<template>
  <div v-if="embedUrls.length" class="previews" :style="mediaScaleStyle">
    <div
      v-for="embedUrl in embedUrls"
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
</template>

<script setup lang="ts">
interface Props {
  embedUrls: string[]
  mediaScalePercent: number
}

const props = defineProps<Props>()

const mediaScaleStyle = computed(() => ({
  '--media-scale': String(props.mediaScalePercent / 100),
}))
</script>

<style scoped>
.previews {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.youtube-preview {
  width: min(100%, calc(480px * var(--media-scale, 1)));
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

@media (max-width: 600px) {
  .youtube-preview {
    max-width: 100%;
    width: 100%;
  }
}
</style>
