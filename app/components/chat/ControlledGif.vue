<template>
  <div
    class="controlled-gif-wrapper"
    @mouseenter="onHoverStart"
    @mouseleave="onHoverEnd"
  >
    <video
      v-if="isVideoSource"
      ref="videoEl"
      :src="proxiedSource"
      class="controlled-gif"
      muted
      loop
      playsinline
      preload="metadata"
      @loadedmetadata="onVideoLoaded"
    />
    <img
      v-else
      ref="gifEl"
      :src="proxiedSource"
      class="controlled-gif"
      loading="lazy"
      decoding="async"
      alt="GIF preview"
    >
  </div>
</template>

<script setup lang="ts">
import { isVideoUrl, normalizeAnimatedUrl, proxyMediaUrl } from '~/composables/useMediaUrl'

interface Props {
  src: string
  playOnHover?: boolean
}

interface FreezeframeInstance {
  start?: () => void
  stop?: () => void
  destroy?: () => void
}

const props = withDefaults(defineProps<Props>(), {
  playOnHover: true,
})

const emit = defineEmits<{
  play: []
  pause: []
}>()

const videoEl = ref<HTMLVideoElement>()
const gifEl = ref<HTMLImageElement>()
const isPlaying = ref(false)
let freezeframeInstance: FreezeframeInstance | null = null

function proxyUrl(url: string) {
  return proxyMediaUrl(url)
}

const normalizedSource = computed(() => normalizeAnimatedUrl(props.src))
const proxiedSource = computed(() => proxyUrl(normalizedSource.value))
const isVideoSource = computed(() => isVideoUrl(normalizedSource.value))

function destroyFreezeframe() {
  freezeframeInstance?.destroy?.()
  freezeframeInstance = null
}

async function setupFreezeframe() {
  if (!import.meta.client || isVideoSource.value) return
  await nextTick()

  const element = gifEl.value
  if (!element) return

  destroyFreezeframe()

  const module = await import('freezeframe')
  const Freezeframe = module.default
  freezeframeInstance = new Freezeframe(element, {
    trigger: false,
    overlay: false,
    responsive: true,
    warnings: false,
  })

  freezeframeInstance.stop?.()
}

function onVideoLoaded() {
  if (!isVideoSource.value) return
  const video = videoEl.value
  if (!video) return
  video.pause()
  video.currentTime = 0
}

function onHoverStart() {
  if (!props.playOnHover) return

  if (isVideoSource.value) {
    const video = videoEl.value
    if (!video) return
    void video.play().then(() => {
      isPlaying.value = true
      emit('play')
    }).catch(() => {
      isPlaying.value = false
      emit('pause')
    })
    return
  }

  freezeframeInstance?.start?.()
  isPlaying.value = true
  emit('play')
}

function onHoverEnd() {
  if (!props.playOnHover) return

  if (isVideoSource.value) {
    const video = videoEl.value
    if (!video) return
    video.pause()
    isPlaying.value = false
    emit('pause')
    return
  }

  freezeframeInstance?.stop?.()
  isPlaying.value = false
  emit('pause')
}

watch(
  () => props.src,
  async () => {
    isPlaying.value = false
    emit('pause')

    if (isVideoSource.value) {
      destroyFreezeframe()
      await nextTick()
      const video = videoEl.value
      if (!video) return
      video.pause()
      video.currentTime = 0
      return
    }

    await setupFreezeframe()
  },
)

onMounted(async () => {
  emit('pause')
  if (!isVideoSource.value) {
    await setupFreezeframe()
  }
})

onBeforeUnmount(() => {
  destroyFreezeframe()
})
</script>

<style scoped>
.controlled-gif-wrapper {
  width: 100%;
  display: inline-block;
  line-height: 0;
}

.controlled-gif {
  width: 100%;
  height: auto;
  display: block;
}
</style>
