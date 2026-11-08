<script setup lang="ts">
import {
  youtubeEmbedUrl,
  youtubeThumbnail,
  youtubeWatchUrl,
  type YouTubeVideo,
} from '../../shared/utils/links'
/**
 * A YouTube video: its thumbnail, title and channel, and a play button. Nothing from YouTube runs
 * until someone presses play; then the privacy-enhanced player (youtube-nocookie.com) is built
 * from the validated video ID, never from page markup. The player is removed again once it is
 * scrolled well out of view, so off-screen videos stop. Videos whose owner disallows embedding
 * keep their card with a link to YouTube instead of a player.
 */
const props = withDefaults(
  defineProps<{
    video: YouTubeVideo
    /** From the server preview, when the channel has server previews on. */
    title?: string
    author?: string
    /** False only when the server learned the owner disallows embedding. */
    embeddable?: boolean
  }>(),
  { title: undefined, author: undefined, embeddable: true },
)
const playing = ref(false)
const failed = ref(false)
const loaded = ref(false)
const frame = ref<HTMLElement>()
const watchUrl = computed(() => youtubeWatchUrl(props.video))
const name = computed(() => props.title || (props.video.short ? 'YouTube Short' : 'YouTube video'))
const blocked = computed(() => props.embeddable === false)

let observer: IntersectionObserver | undefined
function play() {
  if (blocked.value) return
  playing.value = true
  void nextTick(() => {
    if (!frame.value || typeof IntersectionObserver === 'undefined') return
    observer?.disconnect()
    observer = new IntersectionObserver(
      ([entry]) => {
        if (entry && !entry.isIntersecting) stop()
      },
      { rootMargin: '100px' },
    )
    observer.observe(frame.value)
  })
}
function stop() {
  playing.value = false
  observer?.disconnect()
  observer = undefined
}
onBeforeUnmount(stop)
watch(
  () => props.video.id,
  () => {
    stop()
    failed.value = false
    loaded.value = false
  },
)
</script>
<template>
  <a
    v-if="failed"
    class="media-unavailable"
    :href="watchUrl"
    target="_blank"
    rel="noopener noreferrer"
    >Video unavailable · YouTube</a
  >
  <figure
    v-else
    class="embed-card"
    :class="{ 'is-short': video.short }"
  >
    <figcaption class="embed-card-text">
      <small
        >YouTube<template v-if="author"> · {{ author }}</template></small
      >
      <a
        :href="watchUrl"
        target="_blank"
        rel="noopener noreferrer"
        ><strong>{{ name }}</strong></a
      >
    </figcaption>
    <div
      ref="frame"
      class="embed-frame"
      :class="{ 'is-loading': !loaded && !playing }"
    >
      <iframe
        v-if="playing"
        :src="youtubeEmbedUrl(video)"
        :title="`${name} (YouTube player)`"
        allow="autoplay; encrypted-media; picture-in-picture; fullscreen"
        allowfullscreen
        referrerpolicy="strict-origin-when-cross-origin"
        sandbox="allow-scripts allow-same-origin allow-presentation allow-popups"
      />
      <template v-else>
        <img
          :src="youtubeThumbnail(video.id)"
          alt=""
          loading="lazy"
          decoding="async"
          referrerpolicy="no-referrer"
          @load="loaded = true"
          @error="failed = true"
        />
        <button
          v-if="!blocked"
          class="embed-play"
          :aria-label="`Play ${name} from YouTube`"
          @click="play"
        >
          <AppIcon
            name="play"
            :size="26"
          />
        </button>
        <a
          v-else
          class="embed-blocked"
          :href="watchUrl"
          target="_blank"
          rel="noopener noreferrer"
          >Playback is disabled outside YouTube · Watch on YouTube</a
        >
      </template>
    </div>
  </figure>
</template>
