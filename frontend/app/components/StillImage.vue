<script setup lang="ts">
/**
 * An image that holds a still frame unless `playing`. Browsers cannot pause an animated <img>,
 * so once it loads its current (first) frame is copied to a canvas shown in its place, and the
 * <img> is hidden so the browser can skip animating it. A static image looks the same either way,
 * which is why every message image goes through here: an animated WebP or PNG cannot be told
 * apart by its URL. A cross-origin image taints the canvas, which only stops reading its pixels.
 */
const props = defineProps<{
  src: string
  playing: boolean
  alt?: string
  width?: number
  height?: number
  /** Images from other sites load lazily and without a referrer. */
  external?: boolean
}>()
const emit = defineEmits<{ load: []; error: [] }>()
const canvas = ref<HTMLCanvasElement>()
const loaded = ref(false)
watch(
  () => props.src,
  () => (loaded.value = false),
)
/** Caps the still's backing store; it is shown at most 400 CSS pixels wide. */
const maxStill = 960

function still(event: Event) {
  const image = event.target as HTMLImageElement
  const target = canvas.value
  if (target && image.naturalWidth) {
    const scale = Math.min(1, maxStill / Math.max(image.naturalWidth, image.naturalHeight))
    target.width = Math.round(image.naturalWidth * scale)
    target.height = Math.round(image.naturalHeight * scale)
    target.getContext('2d')?.drawImage(image, 0, 0, target.width, target.height)
  }
  loaded.value = true
  emit('load')
}
</script>
<template>
  <img
    :class="{ 'media-hidden': !loaded || !playing }"
    :src="src"
    :alt="alt ?? ''"
    :width="width || undefined"
    :height="height || undefined"
    :loading="external ? 'lazy' : undefined"
    :referrerpolicy="external ? 'no-referrer' : undefined"
    decoding="async"
    @load="still"
    @error="emit('error')"
  />
  <canvas
    v-show="loaded && !playing"
    ref="canvas"
    class="media-still"
    aria-hidden="true"
  />
</template>
