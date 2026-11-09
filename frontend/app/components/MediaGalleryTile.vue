<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { isUploadImageUrl, uploadImageDensities } from '../../shared/utils/upload-image'

/**
 * One cell of a media mosaic: a cropped photo, GIF still, or video frame that opens the viewer.
 * The cell's size comes from the mosaic, so loading, failure, and retry never move the timeline.
 */
const props = defineProps<{
  asset: components['schemas']['LinkMediaAsset']
  position: number
  total: number
  nearby: boolean
  label: string
  /** Items from this one to the end, counted on the last slot when the mosaic can't show them all. */
  more?: number
}>()
const emit = defineEmits<{ open: []; load: []; error: [] }>()
const { t, n } = useI18n()
const appearance = useAppearanceStore()
const { source } = useOfflineMedia(() => props.asset.url)
/** Once a tile starts loading it keeps its media when it scrolls away. */
const started = ref(props.nearby)
watch(
  () => props.nearby,
  (value) => {
    if (value) started.value = true
  },
)
const loaded = ref(false)
const failed = ref(false)
/** Remounts the media after a failure. */
const attempt = ref(0)
watch(source, () => {
  loaded.value = false
  failed.value = false
})
const gif = computed(() => {
  if (props.asset.contentType === 'image/gif') return true
  try {
    return /\.gif$/i.test(new URL(props.asset.url).pathname)
  } catch {
    return false
  }
})
const type = computed(() =>
  gif.value
    ? t('mediaUi.mediaTypes.gif')
    : props.asset.kind === 'video'
      ? t('mediaUi.mediaTypes.video')
      : '',
)
const width = computed(() => (isUploadImageUrl(source.value ?? '') ? 480 : undefined))
const hovered = ref(false)
const focused = ref(false)
const playing = computed(
  () => appearance.animate && props.nearby && (hovered.value || focused.value),
)
const name = computed(() => {
  if (failed.value)
    return t('mediaUi.gallery.retryItem', { current: n(props.position), total: n(props.total) })
  const item = t('mediaUi.gallery.openItem', {
    current: n(props.position),
    total: n(props.total),
    type: type.value || t('mediaUi.mediaTypes.image'),
  })
  return props.more && props.more > 1
    ? t('mediaUi.gallery.openRest', { current: n(props.position), total: n(props.total) })
    : item
})
function ready() {
  loaded.value = true
  emit('load')
}
function broken() {
  loaded.value = false
  failed.value = true
  emit('error')
}
function activate() {
  if (!failed.value) return emit('open')
  failed.value = false
  attempt.value++
}
function enter(event: PointerEvent) {
  if (event.pointerType !== 'touch') hovered.value = true
}
function focus(event: FocusEvent) {
  focused.value = (event.target as Element).matches(':focus-visible')
}
</script>
<template>
  <button
    type="button"
    class="media-gallery-tile group/tile relative block w-full h-full min-w-0 min-h-0 overflow-hidden rounded-[4px] p-0 bg-secondary cursor-zoom-in focus-visible:outline-offset-[-2px] [&.is-failed]:cursor-pointer"
    :class="{ 'is-failed': failed }"
    :aria-label="name"
    :title="name"
    @click="activate"
    @pointerenter="enter"
    @pointerleave="hovered = false"
    @focus="focus"
    @blur="focused = false"
  >
    <span
      v-if="failed"
      class="absolute inset-0 grid place-content-center justify-items-center gap-1.5 p-2 text-center text-[length:var(--ui-caption)] text-muted-foreground"
      role="status"
    >
      <AppIcon
        name="refresh"
        :size="18"
      /><span class="line-clamp-2">{{ $t('mediaUi.gallery.failed') }}</span>
    </span>
    <template v-else-if="started && source">
      <span
        v-if="asset.kind === 'image' && gif"
        :key="`${source}-${attempt}`"
        class="absolute inset-0 [&>img]:size-full [&>img]:object-cover"
        ><StillImage
          :src="source"
          :playing="playing"
          :alt="label"
          :width="asset.width"
          :height="asset.height"
          external
          @load="ready"
          @error="broken"
      /></span>
      <NuxtImg
        v-else-if="asset.kind === 'image'"
        :key="`${source}-${attempt}`"
        class="absolute inset-0 size-full object-cover opacity-0 transition-opacity duration-150 motion-reduce:transition-none [&.is-loaded]:opacity-100"
        :class="{ 'is-loaded': loaded }"
        :src="source"
        :alt="label"
        :width="width"
        :densities="uploadImageDensities(source)"
        loading="lazy"
        decoding="async"
        referrerpolicy="no-referrer"
        @load="ready"
        @error="broken"
      />
      <!-- A frame, not a player: the viewer plays it with sound and controls. -->
      <video
        v-else-if="asset.kind === 'video'"
        :key="`${source}-${attempt}`"
        class="absolute inset-0 size-full object-cover bg-black pointer-events-none"
        :src="source"
        muted
        playsinline
        preload="metadata"
        tabindex="-1"
        aria-hidden="true"
        @loadeddata="ready"
        @error="broken"
      />
    </template>
    <span
      v-if="type && !failed"
      class="pointer-events-none absolute bottom-1.5 left-1.5 inline-flex items-center gap-1 rounded-sm bg-black/65 px-1.5 py-0.5 text-[length:var(--ui-label)] font-semibold leading-[1.4] text-white"
      aria-hidden="true"
      >{{ type
      }}<AppIcon
        :name="gif && playing ? 'pause' : 'play'"
        :size="11"
    /></span>
    <span
      v-if="more && !failed"
      class="pointer-events-none absolute inset-0 grid place-items-center bg-black/55 text-[22px] font-semibold tabular-nums text-white"
      aria-hidden="true"
      >+{{ n(more) }}</span
    >
    <span
      class="pointer-events-none absolute inset-0 bg-white/0 transition-colors duration-150 group-hover/tile:bg-white/6 motion-reduce:transition-none"
      aria-hidden="true"
    />
  </button>
</template>
