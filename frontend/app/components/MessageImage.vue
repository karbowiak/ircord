<script setup lang="ts">
import { sniffAnimated } from '../../shared/utils/animated'
/**
 * A linked or uploaded image in a message. It holds a still frame unless playback is allowed
 * (see useMediaPlayback). Images known to be animated get a GIF label and take a first tap on
 * touch screens to play; same-origin files are checked by their bytes, other sites by URL.
 */
defineOptions({ inheritAttrs: false })
const props = defineProps<{
  src: string
  /** Accessible name of the button that opens the viewer. */
  label: string
  alt?: string
  width?: number
  height?: number
  /** Known to be animated from its URL or content type. */
  animated?: boolean
  /** Linked images come from other sites: lazy, without a referrer. Uploads load from this app. */
  external?: boolean
  /** Shown as a link when the image fails to load; without it, `error` is left to the caller. */
  unavailable?: string
}>()
const emit = defineEmits<{ open: []; error: [] }>()
const { playing, handlers, claimTap } = useMediaPlayback()
const loaded = ref(false)
const failed = ref(false)
const sniffed = ref(false)
const isAnimated = computed(() => props.animated || sniffed.value)
watch(
  () => props.src,
  (src) => {
    loaded.value = false
    failed.value = false
    sniffed.value = false
    if (!props.animated && new URL(src, location.href).origin === location.origin)
      void sniffAnimated(src).then((found) => {
        if (src === props.src) sniffed.value = found
      })
  },
  { immediate: true },
)

function open() {
  if (!(isAnimated.value && claimTap())) emit('open')
}
function fail() {
  failed.value = true
  emit('error')
}
</script>
<template>
  <a
    v-if="failed && unavailable"
    class="media-unavailable"
    :href="src"
    target="_blank"
    rel="noopener noreferrer"
    >{{ unavailable }}</a
  >
  <button
    v-else
    v-bind="$attrs"
    class="message-image media-frame"
    :class="{ 'is-loading': !loaded && !failed, 'is-playing': playing }"
    :aria-label="label"
    :aria-busy="!loaded && !failed"
    v-on="handlers"
    @click="open"
  >
    <StillImage
      :src="src"
      :alt="alt"
      :width="width"
      :height="height"
      :external="external"
      :playing="playing"
      @load="loaded = true"
      @error="fail"
    />
    <span
      v-if="isAnimated"
      class="media-kind"
      aria-hidden="true"
      >GIF</span
    >
  </button>
</template>
