<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { publicMediaURL } from '../../shared/utils/link-media'
import { isUploadImageUrl, uploadImageDensities } from '../../shared/utils/upload-image'
import { offlineStatus } from '../utils/offline-archive'

const props = defineProps<{
  asset: components['schemas']['LinkMediaAsset']
  nearby: boolean
  description?: string
  /** The posted link stays separate from a cached media or image-variant URL. */
  url?: string
  /** False when the surrounding embed shows its own copy and open actions. */
  actions?: boolean
  /** Inline image/GIF adapters have no painted outer frame. */
  plain?: boolean
}>()
const emit = defineEmits<{ unavailable: []; ready: [ready: boolean]; viewing: [open: boolean] }>()
const appearance = useAppearanceStore()
const root = ref<HTMLElement | null>(null)
const player = ref<HTMLVideoElement | null>(null)
const hovered = ref(false)
const focused = ref(false)
const explicit = ref<boolean | null>(null)
const visible = ref(!document.hidden)
const expanded = ref(false)
watch(expanded, (open) => emit('viewing', open))
const { t } = useI18n()
const { source, loading } = useOfflineMedia(() => publicMediaURL(props.asset.url))
const displaySize = computed(() => {
  const { width, height } = props.asset
  if (!isUploadImageUrl(source.value))
    return { width: width || undefined, height: height || undefined }
  const displayWidth = Math.min(width || 480, 480)
  return {
    width: displayWidth,
    height: width && height ? Math.max(1, Math.round((displayWidth * height) / width)) : undefined,
  }
})
const original = computed(() => props.url ?? props.asset.url)
const renderedSource = ref<string>()
const ready = computed(
  () => !!source.value && renderedSource.value === source.value && props.asset.kind !== 'audio',
)
watch(ready, (value) => emit('ready', value), { immediate: true })
function loaded() {
  renderedSource.value = source.value
}
function videoLoaded() {
  loaded()
  syncPlayer()
}
function unavailable() {
  renderedSource.value = undefined
  emit('unavailable')
}
const gif = computed(
  () =>
    props.asset.contentType === 'image/gif' || /\.gif$/i.test(new URL(props.asset.url).pathname),
)
const moving = computed(() => gif.value || props.asset.kind === 'video')
const label = computed(() =>
  t(gif.value ? 'mediaUi.mediaTypes.gif' : `mediaUi.mediaTypes.${props.asset.kind}`),
)
const playing = computed(
  () =>
    !expanded.value &&
    props.nearby &&
    visible.value &&
    (explicit.value ?? (appearance.animate && (hovered.value || focused.value))),
)
/**
 * A fixed box: the parent's reserved `--embed-height`, or a fixed stage of its own. Media is
 * contained inside it, so loading, decoding, video metadata and failure never change its size.
 */
const box = computed(() => ({
  height:
    props.asset.kind === 'audio'
      ? 'var(--embed-height, 88px)'
      : 'var(--embed-height, clamp(220px, 75vw, 400px))',
}))

function pause() {
  hovered.value = false
  focused.value = false
  explicit.value = null
  player.value?.pause()
}
function enter(event: PointerEvent) {
  if (event.pointerType === 'touch') return
  explicit.value = null
  hovered.value = true
}
function leave(event: PointerEvent) {
  if (event.pointerType !== 'touch') pause()
}
function focus(event: FocusEvent) {
  focused.value = (event.target as Element).matches(':focus-visible')
  explicit.value = null
}
function blur(event: FocusEvent) {
  if (!root.value?.contains(event.relatedTarget as Node | null)) pause()
}
function openPreview() {
  expanded.value = true
}
function visibility() {
  visible.value = !document.hidden
  if (!visible.value) pause()
}
function syncPlayer() {
  const video = player.value
  if (!video) return
  if (playing.value) {
    video.muted = true
    void video.play().catch(() => {}) // A blocked preview remains a normal player.
  } else video.pause()
}
watch([playing, player], syncPlayer)
watch(
  () => props.nearby,
  (value) => {
    if (!value) pause()
  },
)
watch([source, () => props.asset.kind], () => {
  renderedSource.value = undefined
  pause()
})
onMounted(() => document.addEventListener('visibilitychange', visibility))
onBeforeUnmount(() => {
  pause()
  emit('ready', false)
  emit('viewing', false)
  document.removeEventListener('visibilitychange', visibility)
})
</script>
<template>
  <div
    ref="root"
    class="direct-media group/media relative w-full min-w-0 overflow-hidden rounded-[12px] bg-card [&.is-audio]:grid [&.is-audio]:content-end [&.is-audio]:px-2 [&.is-audio]:pt-8 [&.is-audio]:pb-2"
    :class="{
      'is-audio': asset.kind === 'audio',
      'bg-transparent! rounded-lg! [&_.media-still]:bg-transparent! [&_.media-still]:object-left-top [&_.direct-media-frame>img]:object-left-top':
        plain,
    }"
    :style="box"
    role="group"
    :aria-label="t('mediaUi.directMedia.preview', { type: label })"
    @pointerenter="enter"
    @pointerleave="leave"
    @focusin="focus"
    @focusout="blur"
  >
    <p
      v-if="offlineStatus.offline && !source && !loading"
      class="absolute inset-0 m-0 grid place-items-center p-3 text-center text-[length:var(--ui-caption)] text-muted-foreground"
    >
      {{ $t('mediaUi.directMedia.offlineNotSaved') }}
    </p>
    <!-- The old direct-media rules in link-media.css (height:auto, max-height:380px) no longer
         match these class names; size comes only from the box above. -->
    <button
      v-if="asset.kind === 'image' && source"
      type="button"
      class="direct-media-frame relative block size-full p-0 bg-transparent cursor-zoom-in [&>img]:size-full [&>img]:object-contain [&_.media-still]:object-contain"
      :aria-label="$t('mediaUi.directMedia.openImage')"
      @click="openPreview"
    >
      <StillImage
        v-if="gif && source"
        :key="source"
        :src="source"
        :playing="playing"
        :alt="description || $t('mediaUi.directMedia.gifPreview')"
        :width="asset.width"
        :height="asset.height"
        external
        @load="loaded"
        @error="unavailable"
      />
      <NuxtImg
        v-else-if="source"
        :key="source"
        :src="source"
        :alt="description || $t('mediaUi.directMedia.linkedImage')"
        :width="displaySize.width"
        :height="displaySize.height"
        :densities="uploadImageDensities(source)"
        loading="lazy"
        decoding="async"
        referrerpolicy="no-referrer"
        @load="loaded"
        @error="unavailable"
      />
    </button>
    <video
      v-else-if="asset.kind === 'video' && source"
      :key="source"
      ref="player"
      class="block size-full object-contain bg-black"
      :src="source"
      controls
      muted
      playsinline
      :preload="nearby ? 'metadata' : 'none'"
      :aria-label="description || $t('mediaUi.directMedia.linkedVideo')"
      @play="explicit = true"
      @pause="explicit = false"
      @loadeddata="videoLoaded"
      @error="unavailable"
    />
    <audio
      v-else-if="asset.kind === 'audio' && source"
      class="block w-full"
      :src="source"
      controls
      preload="metadata"
      :aria-label="$t('mediaUi.directMedia.linkedAudio')"
      @error="unavailable"
    />
    <span
      v-if="source"
      class="direct-media-label absolute top-2 left-2 inline-flex items-center gap-1 py-[3px] px-1.5 border border-solid border-border rounded-sm bg-card text-foreground text-[10px] font-[600] leading-[1.4] pointer-events-none"
      >{{ label
      }}<AppIcon
        v-if="moving"
        :name="playing ? 'pause' : 'play'"
        :size="11"
    /></span>
    <MediaLinkActions
      v-if="actions !== false && asset.kind !== 'audio'"
      :url="original"
      always-visible
      class="absolute top-2 right-2 z-2"
    />
    <MediaViewer
      v-if="expanded"
      :items="[asset]"
      :label="description || $t('mediaUi.directMedia.linkedImage')"
      :source="original"
      @close="expanded = false"
    />
  </div>
</template>
