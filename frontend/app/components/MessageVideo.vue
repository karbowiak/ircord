<script setup lang="ts">
/**
 * A linked video. It stays paused on its first frame unless playback is allowed (autoplay, or a
 * hover/focus preview), and then plays muted and looped. Autoplay runs only while the video is
 * near the viewport, so scrolled-away videos stop decoding and downloading. The native controls
 * remain the touch way to play, and the way to play with sound. Once someone plays, pauses or
 * unmutes with them, the video is theirs: hover, focus and scrolling no longer touch it.
 * Media elements have no referrerpolicy attribute; requests follow the app's document policy.
 */
const props = defineProps<{ src: string; host: string }>()
const { autoplay, playing, handlers } = useMediaPlayback()
const video = ref<HTMLVideoElement>()
const visible = ref(false)
const loaded = ref(false)
const failed = ref(false)
/** True while this component, not the user's own controls, started playback. */
let started = false
/** The play or pause event this component's own call will cause; any other one is the user's. */
let expecting: 'play' | 'pause' | null = null
const userOwned = ref(false)
function takeOver() {
  userOwned.value = true
  started = false
  expecting = null
}
function onPlay() {
  if (expecting === 'play') expecting = null
  else takeOver()
}
function onPause() {
  // A preview that is not looped ends by itself; that is not the user pausing.
  if (video.value?.ended && started) started = false
  else if (expecting === 'pause') expecting = null
  else takeOver()
}
function onVolumeChange() {
  if (video.value && !video.value.muted) takeOver()
}

let observer: IntersectionObserver | undefined
onMounted(() => {
  if (!video.value || typeof IntersectionObserver === 'undefined') return
  observer = new IntersectionObserver(([entry]) => (visible.value = !!entry?.isIntersecting), {
    rootMargin: '200px',
  })
  observer.observe(video.value)
})
onBeforeUnmount(() => observer?.disconnect())

watch(
  () => !userOwned.value && playing.value && (visible.value || !autoplay.value),
  (play) => {
    const el = video.value
    if (!el || userOwned.value) return
    if (play && el.paused) {
      el.muted = true
      started = true
      expecting = 'play'
      void el.play().catch(() => {
        started = false
        expecting = null
      })
    } else if (!play && started) {
      started = false
      if (!el.paused) {
        expecting = 'pause'
        el.pause()
      }
    }
  },
)
watch(
  () => props.src,
  () => {
    failed.value = false
    loaded.value = false
    userOwned.value = false
    started = false
    expecting = null
  },
)
</script>
<template>
  <a
    v-if="failed"
    class="media-unavailable"
    :href="src"
    target="_blank"
    rel="noopener noreferrer"
    >Video unavailable · {{ host }}</a
  >
  <div
    v-else
    class="link-preview media-frame"
    :class="{ 'is-loading': !loaded, 'is-playing': playing }"
    :aria-busy="!loaded"
    v-on="handlers"
  >
    <video
      ref="video"
      class="link-video"
      :src="src"
      controls
      :loop="autoplay"
      :preload="autoplay && visible ? 'auto' : 'metadata'"
      playsinline
      :aria-label="`Video from ${host}`"
      @loadedmetadata="loaded = true"
      @play="onPlay"
      @pause="onPause"
      @volumechange="onVolumeChange"
      @error="failed = true"
    />
    <span
      class="media-kind"
      aria-hidden="true"
      >Video</span
    >
  </div>
</template>
