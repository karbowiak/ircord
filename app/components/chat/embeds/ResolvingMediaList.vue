<template>
  <div v-if="urls.length" class="previews" :style="mediaScaleStyle">
    <div
      v-for="loadingUrl in urls"
      :key="loadingUrl"
      class="media-resolving-card"
    >
      <span class="media-resolving-spinner" aria-hidden="true" />
      <span class="media-resolving-text">Resolving media...</span>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  urls: string[]
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

.media-resolving-card {
  width: min(100%, calc(480px * var(--media-scale, 1)));
  min-height: 88px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.media-resolving-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(255, 255, 255, 0.24);
  border-top-color: rgba(255, 255, 255, 0.8);
  border-radius: 999px;
  animation: media-spin 0.8s linear infinite;
}

.media-resolving-text {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text-muted);
}

@keyframes media-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
