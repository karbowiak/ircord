<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { publicMediaURL } from '../../shared/utils/link-media'
import { offlineStatus } from '../utils/offline-archive'

const props = defineProps<{
  asset: components['schemas']['LinkMediaAsset']
  nearby: boolean
  description?: string
}>()
const emit = defineEmits<{ unavailable: [] }>()
const appearance = useAppearanceStore()
const root = ref<HTMLElement | null>(null)
const player = ref<HTMLVideoElement | null>(null)
const hovered = ref(false)
const focused = ref(false)
const explicit = ref<boolean | null>(null)
const visible = ref(!document.hidden)
const expanded = ref(false)
const { source, loading } = useOfflineMedia(() => publicMediaURL(props.asset.url))
const gif = computed(
  () =>
    props.asset.contentType === 'image/gif' || /\.gif$/i.test(new URL(props.asset.url).pathname),
)
const moving = computed(() => gif.value || props.asset.kind === 'video')
const label = computed(() => (gif.value ? 'GIF' : props.asset.kind.toUpperCase()))
const playing = computed(
  () =>
    props.nearby &&
    visible.value &&
    (explicit.value ?? (appearance.animate && (hovered.value || focused.value))),
)
const ratio = computed(() =>
  props.asset.width && props.asset.height
    ? `${props.asset.width}/${props.asset.height}`
    : undefined,
)
/** Known media narrows the frame to its own shape instead of sitting between bars. */
const fit = computed(() =>
  props.asset.width && props.asset.height && props.asset.kind !== 'audio'
    ? { '--media-ratio': props.asset.width / props.asset.height }
    : undefined,
)

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
function toggle() {
  if (gif.value) explicit.value = !playing.value
  else expanded.value = true
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
watch(source, pause)
onMounted(() => document.addEventListener('visibilitychange', visibility))
onBeforeUnmount(() => {
  pause()
  document.removeEventListener('visibilitychange', visibility)
})
</script>
<template>
  <div
    ref="root"
    class="direct-link-media relative overflow-hidden rounded-[12px] bg-card"
    :class="{ 'is-audio': asset.kind === 'audio', 'is-fitted': fit }"
    :style="fit"
    :aria-label="`${label} preview`"
    @pointerenter="enter"
    @pointerleave="leave"
    @focusin="focus"
    @focusout="blur"
  >
    <p
      v-if="offlineStatus.offline && !source && !loading"
      class="px-3 py-3 text-[length:var(--ui-caption)] text-muted-foreground"
      role="status"
    >
      This media was not saved on this device.
    </p>
    <button
      v-if="asset.kind === 'image' && source"
      type="button"
      class="direct-media-image relative block w-full p-0 min-h-25 max-h-95 bg-transparent"
      :style="{ aspectRatio: ratio }"
      :aria-label="gif ? `${playing ? 'Pause' : 'Play'} GIF preview` : 'Open image'"
      :aria-pressed="gif ? playing : undefined"
      @click="toggle"
    >
      <StillImage
        v-if="gif && source"
        :src="source"
        :playing="playing"
        :alt="description || 'GIF preview'"
        :width="asset.width"
        :height="asset.height"
        external
        @error="emit('unavailable')"
      />
      <NuxtImg
        v-else-if="source"
        :src="source"
        :alt="description || 'Linked image'"
        :width="asset.width || undefined"
        :height="asset.height || undefined"
        loading="lazy"
        decoding="async"
        referrerpolicy="no-referrer"
        @error="emit('unavailable')"
      />
    </button>
    <video
      v-else-if="asset.kind === 'video' && source"
      ref="player"
      :src="source"
      :style="{ aspectRatio: ratio || '16/9' }"
      controls
      muted
      playsinline
      :preload="nearby ? 'metadata' : 'none'"
      :aria-label="description || 'Linked video'"
      @play="explicit = true"
      @pause="explicit = false"
      @loadeddata="syncPlayer"
      @error="emit('unavailable')"
    />
    <audio
      v-else-if="asset.kind === 'audio' && source"
      :src="source"
      controls
      preload="metadata"
      aria-label="Linked audio"
      @error="emit('unavailable')"
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
    <MediaViewer
      v-if="expanded"
      :items="[asset]"
      :label="description || 'Linked image'"
      :source="asset.url"
      @close="expanded = false"
    />
  </div>
</template>
