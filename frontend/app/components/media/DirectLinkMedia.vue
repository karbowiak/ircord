<script setup vapor lang="ts">
import type { components } from '../../../shared/generated/api'
import { publicMediaURL } from '../../../shared/utils/link-media'
import { isUploadImageUrl, uploadImageDensities } from '../../../shared/utils/upload-image'
import { offlineStatus } from '../../utils/offline-archive'

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
const player = ref<HTMLMediaElement | null>(null)
const hovered = ref(false)
const focused = ref(false)
const visible = ref(!document.hidden)
const expanded = ref(false)
useViewHistory(`direct-media:${useId()}`, expanded)
watch(expanded, (open) => emit('viewing', open))
const { t } = useI18n()
const { source, loading } = useOfflineMedia(() => publicMediaURL(props.asset.url))
const displaySize = computed(() => {
  const { width, height } = props.asset
  if (!isUploadImageUrl(source.value))
    return { width: width || undefined, height: height || undefined }
  // A 400px-tall landscape image can be much wider than the old card width. Request a
  // useful display copy without changing capabilities or third-party source URLs.
  const displayWidth = Math.min(width || 1280, 1280)
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
  playback.sync()
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
const active = computed(() => !expanded.value && props.nearby && visible.value)
const preview = computed(() => appearance.animate && (hovered.value || focused.value))
const playback = useInlinePlayback(
  player,
  active,
  () => props.asset.kind === 'video' && preview.value,
  unavailable,
)
const playing = computed(() => (gif.value ? active.value && preview.value : playback.playing.value))

const currentTime = ref(0)
const duration = ref(0)
const isMuted = ref(props.asset.kind === 'video')
const scrubbing = ref(false)
const scrubTrack = ref<HTMLElement | null>(null)

function formatMediaTime(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const total = Math.floor(seconds)
  const hrs = Math.floor(total / 3600)
  const mins = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (hrs > 0) return `${hrs}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
  return `${mins}:${String(secs).padStart(2, '0')}`
}

const mediaTitle = computed(() => {
  if (props.description?.trim()) return props.description.trim()
  const raw = props.url ?? props.asset.url
  try {
    const segment = decodeURIComponent(
      new URL(raw, window.location.href).pathname.split('/').pop() || '',
    )
    if (segment) return segment
  } catch {
    // Fall back to filename slice when URL parsing fails.
  }
  return raw.split('/').pop()?.split('?')[0] || label.value
})

const mediaFormatBadge = computed(() => {
  const candidates = [props.url, props.asset.url]
  for (const raw of candidates) {
    if (!raw) continue
    try {
      const path = new URL(raw, window.location.href).pathname
      const match = path.match(/\.([a-z0-9]{2,5})$/i)
      if (match?.[1]) return match[1].toUpperCase()
    } catch {
      const match = raw.split('?')[0]?.match(/\.([a-z0-9]{2,5})$/i)
      if (match?.[1]) return match[1].toUpperCase()
    }
  }
  const sub = props.asset.contentType?.split('/')[1]?.split(';')[0]
  if (sub) return sub.replace(/^x-/i, '').toUpperCase()
  return props.asset.kind.toUpperCase()
})

const WAVEFORM_SPECTRUM = [
  'var(--accent)',
  '#f97316',
  '#f59e0b',
  '#eab308',
  '#4ade80',
  '#38bdf8',
] as const

const waveformBars = computed(() => {
  const seedKey = `${props.asset.url}:${mediaTitle.value}`
  let hash = 2166136261
  for (let i = 0; i < seedKey.length; i++) {
    hash ^= seedKey.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  const bars: number[] = []
  for (let i = 0; i < 38; i++) {
    hash ^= i * 37 + 17
    hash = Math.imul(hash, 16777619)
    const normalized = (hash >>> 0) / 4294967295
    // Shape a subtle musical envelope so edges taper naturally.
    const envelope = 0.65 + 0.35 * Math.sin((i / 37) * Math.PI)
    bars.push(Math.max(0.22, Math.min(1, (0.28 + normalized * 0.72) * envelope)))
  }
  return bars
})

const progressRatio = computed(() =>
  duration.value > 0 ? Math.max(0, Math.min(1, currentTime.value / duration.value)) : 0,
)

function syncMediaMetadata() {
  const media = player.value
  if (!media) return
  if (Number.isFinite(media.duration) && media.duration > 0) duration.value = media.duration
  currentTime.value = Number.isFinite(media.currentTime) ? media.currentTime : 0
  isMuted.value = media.muted
}

function handleTimeUpdate() {
  const media = player.value
  if (!media) return
  currentTime.value = Number.isFinite(media.currentTime) ? media.currentTime : 0
  if (Number.isFinite(media.duration) && media.duration > 0) duration.value = media.duration
}

function handleEnded() {
  playback.paused()
  const media = player.value
  if (media) currentTime.value = media.currentTime
}

function handleVolumeChange() {
  const media = player.value
  if (media) isMuted.value = media.muted
  playback.volumeChanged()
}

function togglePlayback() {
  const media = player.value
  if (!media) return
  playback.takeControl()
  if (media.paused) {
    void media.play().catch((error: unknown) => {
      const name = error && typeof error === 'object' && 'name' in error ? error.name : undefined
      if (name !== 'AbortError' && name !== 'NotAllowedError') unavailable()
    })
  } else {
    media.pause()
  }
}

function seekRatio(ratio: number) {
  const media = player.value
  if (!media || !Number.isFinite(media.duration) || media.duration <= 0) return
  playback.takeControl()
  const clamped = Math.max(0, Math.min(1, ratio))
  media.currentTime = clamped * media.duration
  currentTime.value = media.currentTime
}

function ratioFromPointer(event: PointerEvent, target: HTMLElement | null) {
  if (!target) return 0
  const rect = target.getBoundingClientRect()
  if (rect.width <= 0) return 0
  return Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width))
}

function onScrubPointerDown(event: PointerEvent) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) return
  scrubTrack.value = target
  scrubbing.value = true
  target.setPointerCapture?.(event.pointerId)
  seekRatio(ratioFromPointer(event, target))
}

function onScrubPointerMove(event: PointerEvent) {
  if (!scrubbing.value) return
  seekRatio(
    ratioFromPointer(event, scrubTrack.value ?? (event.currentTarget as HTMLElement | null)),
  )
}

function onScrubPointerUp(event: PointerEvent) {
  if (!scrubbing.value) return
  scrubbing.value = false
  const target = event.currentTarget as HTMLElement | null
  if (target?.hasPointerCapture?.(event.pointerId)) target.releasePointerCapture(event.pointerId)
  scrubTrack.value = null
}

function onScrubberKeydown(event: KeyboardEvent) {
  const media = player.value
  if (!media || !Number.isFinite(media.duration) || media.duration <= 0) return
  const step = 5 / media.duration
  if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
    event.preventDefault()
    seekRatio(progressRatio.value - step)
  } else if (event.key === 'ArrowRight' || event.key === 'ArrowUp') {
    event.preventDefault()
    seekRatio(progressRatio.value + step)
  } else if (event.key === 'Home') {
    event.preventDefault()
    seekRatio(0)
  } else if (event.key === 'End') {
    event.preventDefault()
    seekRatio(1)
  }
}

function toggleMute() {
  const media = player.value
  if (!media) return
  playback.takeControl()
  media.muted = !media.muted
  isMuted.value = media.muted
  playback.volumeChanged()
}

function waveformBarColor(index: number) {
  const bucket = Math.min(
    WAVEFORM_SPECTRUM.length - 1,
    Math.floor((index / waveformBars.value.length) * WAVEFORM_SPECTRUM.length),
  )
  return WAVEFORM_SPECTRUM[bucket]
}
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
  playback.pause()
}
function enter(event: PointerEvent) {
  if (event.pointerType === 'touch') return
  hovered.value = true
}
function leave(event: PointerEvent) {
  if (event.pointerType !== 'touch') hovered.value = false
}
function focus(event: FocusEvent) {
  focused.value = (event.target as Element).matches(':focus-visible')
}
function blur(event: FocusEvent) {
  if (!root.value?.contains(event.relatedTarget as Node | null)) focused.value = false
}
function openPreview() {
  pause()
  expanded.value = true
}
function visibility() {
  visible.value = !document.hidden
  if (!visible.value) pause()
}
watch(
  () => props.nearby,
  (value) => {
    if (!value) pause()
  },
)
watch([source, () => props.asset.kind], () => {
  renderedSource.value = undefined
  currentTime.value = 0
  duration.value = 0
  isMuted.value = props.asset.kind === 'video'
  pause()
  playback.reset()
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
    class="direct-media group/media relative w-full min-w-0 overflow-hidden rounded-[12px] bg-card shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_65%,transparent)] [&.is-audio]:flex [&.is-audio]:items-center [&.is-audio]:gap-3 [&.is-audio]:px-3.5 [&.is-audio]:py-2.5 [&.is-audio]:[background:radial-gradient(ellipse_220px_90px_at_10%_50%,color-mix(in_srgb,var(--accent)_22%,transparent),transparent),linear-gradient(180deg,color-mix(in_srgb,var(--raised)_75%,var(--card)),var(--card))] [&.is-audio]:shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_85%,transparent)]"
    :class="{
      'is-audio': asset.kind === 'audio',
      'bg-transparent! shadow-none! rounded-lg! [&_.media-still]:bg-transparent! [&_.media-still]:object-left-top [&_.direct-media-frame>img]:object-left-top':
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
      {{ t('mediaUi.directMedia.offlineNotSaved') }}
    </p>
    <!-- The old direct-media rules in link-media.css (height:auto, max-height:380px) no longer
         match these class names; size comes only from the box above. -->
    <button
      v-if="asset.kind === 'image' && source"
      type="button"
      class="direct-media-frame relative block size-full p-0 bg-transparent cursor-zoom-in [&>img]:size-full [&>img]:object-contain [&_.media-still]:object-contain"
      :aria-label="t('mediaUi.directMedia.openImage')"
      @click="openPreview"
    >
      <StillImage
        v-if="gif && source"
        :key="`gif:${source}`"
        :src="source"
        :playing="playing"
        :alt="description || t('mediaUi.directMedia.gifPreview')"
        :width="asset.width"
        :height="asset.height"
        external
        @load="loaded"
        @error="unavailable"
      />
      <NuxtImg
        v-else-if="source"
        :key="`image:${source}`"
        :src="source"
        :alt="description || t('mediaUi.directMedia.linkedImage')"
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
    <template v-else-if="asset.kind === 'video' && source">
      <video
        :key="source"
        ref="player"
        class="block size-full cursor-pointer object-contain bg-black"
        :src="source"
        muted
        playsinline
        :preload="nearby ? 'metadata' : 'none'"
        :aria-label="description || t('mediaUi.directMedia.linkedVideo')"
        @click="togglePlayback"
        @pointerdown="playback.takeControl"
        @keydown="playback.takeControl"
        @loadedmetadata="syncMediaMetadata"
        @durationchange="syncMediaMetadata"
        @timeupdate="handleTimeUpdate"
        @volumechange="handleVolumeChange"
        @play="playback.played"
        @pause="playback.paused"
        @ended="handleEnded"
        @loadeddata="videoLoaded"
        @error="unavailable"
      />
      <button
        v-if="!playing"
        type="button"
        class="video-center-play absolute left-1/2 top-1/2 z-2 grid size-14 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full bg-black/55 backdrop-blur-md text-white shadow-[inset_0_0_0_1px_rgb(255_255_255/22%),0_8px_24px_rgb(0_0_0/50%)] transition-transform hover:scale-105"
        :aria-label="t('mediaUi.directMedia.playMedia', { type: label })"
        @click.stop="togglePlayback"
      >
        <AppIcon
          name="play"
          :size="24"
          class="translate-x-[1px] text-primary"
        />
      </button>
      <div
        class="video-deck-bar absolute inset-x-0 bottom-0 z-2 flex items-center gap-2.5 bg-gradient-to-t from-black/85 via-black/50 to-transparent px-3 pt-7 pb-2.5"
        @click.stop
        @pointerdown.stop="playback.takeControl"
      >
        <button
          type="button"
          class="inline-flex size-8 shrink-0 items-center justify-center rounded-full [background:linear-gradient(135deg,var(--accent),color-mix(in_srgb,var(--accent)_72%,var(--danger)))] text-white shadow-[inset_0_1px_0_rgb(255_255_255/28%),0_3px_10px_color-mix(in_srgb,var(--accent)_36%,transparent)] transition-transform hover:scale-105"
          :aria-label="
            t(playing ? 'mediaUi.directMedia.pauseMedia' : 'mediaUi.directMedia.playMedia', {
              type: label,
            })
          "
          @click="togglePlayback"
        >
          <AppIcon
            :name="playing ? 'pause' : 'play'"
            :size="14"
            :class="{ 'translate-x-[0.5px]': !playing }"
          />
        </button>
        <div
          class="video-scrubber group/scrub relative flex h-5 min-w-0 flex-1 cursor-pointer items-center touch-none"
          role="slider"
          tabindex="0"
          :aria-label="t('mediaUi.directMedia.seekMedia', { type: label })"
          :aria-valuemin="0"
          :aria-valuemax="Math.round(duration || 100)"
          :aria-valuenow="Math.round(currentTime)"
          @pointerdown="onScrubPointerDown"
          @pointermove="onScrubPointerMove"
          @pointerup="onScrubPointerUp"
          @pointercancel="onScrubPointerUp"
          @keydown="onScrubberKeydown"
        >
          <div class="relative h-1.5 w-full overflow-hidden rounded-full bg-white/20">
            <div
              class="h-full rounded-full [background:linear-gradient(90deg,var(--accent),#f59e0b)] transition-[width] duration-75"
              :style="{ width: `${Math.round(progressRatio * 1000) / 10}%` }"
            />
          </div>
        </div>
        <span class="shrink-0 font-code text-[11px] tabular-nums text-white/90">
          {{ formatMediaTime(currentTime) }} / {{ formatMediaTime(duration) }}
        </span>
        <button
          type="button"
          class="inline-flex size-7 shrink-0 items-center justify-center rounded-full bg-white/10 text-white/90 transition-colors hover:bg-white/20"
          :aria-label="
            t(isMuted ? 'mediaUi.directMedia.unmuteMedia' : 'mediaUi.directMedia.muteMedia', {
              type: label,
            })
          "
          @click="toggleMute"
        >
          <AppIcon
            :name="isMuted ? 'volume-off' : 'volume'"
            :size="14"
          />
        </button>
        <button
          type="button"
          class="inline-flex size-7 shrink-0 items-center justify-center rounded-full bg-white/10 text-white/90 transition-colors hover:bg-white/20"
          :aria-label="t('mediaUi.directMedia.openVideo')"
          @click="openPreview"
        >
          <AppIcon
            name="external"
            :size="13"
          />
        </button>
      </div>
    </template>
    <template v-else-if="asset.kind === 'audio' && source">
      <audio
        ref="player"
        class="sr-only"
        :src="source"
        preload="metadata"
        :aria-label="t('mediaUi.directMedia.linkedAudio')"
        @pointerdown="playback.takeControl"
        @keydown="playback.takeControl"
        @loadedmetadata="syncMediaMetadata"
        @durationchange="syncMediaMetadata"
        @timeupdate="handleTimeUpdate"
        @volumechange="handleVolumeChange"
        @play="playback.played"
        @pause="playback.paused"
        @ended="handleEnded"
        @error="unavailable"
      />
      <div
        class="audio-deck-art relative grid size-14 shrink-0 place-items-center overflow-hidden rounded-lg [background:linear-gradient(145deg,color-mix(in_srgb,var(--accent)_32%,var(--raised)),color-mix(in_srgb,var(--danger)_18%,var(--card)))] shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--accent)_32%,var(--border))]"
      >
        <AppIcon
          name="audio"
          :size="24"
          class="pointer-events-none text-foreground/20"
        />
        <button
          type="button"
          class="audio-play-button absolute inline-flex size-10 items-center justify-center rounded-full [background:linear-gradient(135deg,var(--accent),color-mix(in_srgb,var(--accent)_72%,var(--danger)))] text-white shadow-[inset_0_1px_0_rgb(255_255_255/30%),0_4px_12px_color-mix(in_srgb,var(--accent)_40%,transparent)] transition-transform hover:scale-105"
          :aria-label="
            t(playing ? 'mediaUi.directMedia.pauseMedia' : 'mediaUi.directMedia.playMedia', {
              type: label,
            })
          "
          @click="togglePlayback"
        >
          <AppIcon
            :name="playing ? 'pause' : 'play'"
            :size="16"
            :class="{ 'translate-x-[0.5px]': !playing }"
          />
        </button>
      </div>
      <div class="audio-deck-main flex min-w-0 flex-1 flex-col justify-center gap-1.5">
        <div class="flex min-w-0 items-center justify-between gap-2">
          <strong class="truncate text-[length:var(--ui-small)] font-semibold text-foreground">{{
            mediaTitle
          }}</strong>
          <span
            class="audio-format-pill shrink-0 rounded-full bg-secondary/90 px-2 py-0.5 font-code text-[10px] font-semibold uppercase tracking-[0.04em] text-muted-foreground shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--border)_75%,transparent)]"
            >{{ mediaFormatBadge }}</span
          >
        </div>
        <div class="flex min-w-0 items-center gap-2">
          <span class="shrink-0 font-code text-[11px] tabular-nums text-muted-foreground">{{
            formatMediaTime(currentTime)
          }}</span>
          <div
            class="audio-waveform flex h-6 min-w-0 flex-1 cursor-pointer items-center gap-[2px] touch-none"
            role="slider"
            tabindex="0"
            :aria-label="t('mediaUi.directMedia.seekMedia', { type: label })"
            :aria-valuemin="0"
            :aria-valuemax="Math.round(duration || 100)"
            :aria-valuenow="Math.round(currentTime)"
            @pointerdown="onScrubPointerDown"
            @pointermove="onScrubPointerMove"
            @pointerup="onScrubPointerUp"
            @pointercancel="onScrubPointerUp"
            @keydown="onScrubberKeydown"
          >
            <span
              v-for="(barHeight, idx) in waveformBars"
              :key="idx"
              class="min-w-0 flex-1 rounded-full transition-colors duration-75"
              :class="idx / waveformBars.length <= progressRatio ? '' : 'bg-foreground/20'"
              :style="{
                height: `${Math.round(barHeight * 100)}%`,
                backgroundColor:
                  idx / waveformBars.length <= progressRatio ? waveformBarColor(idx) : undefined,
              }"
            />
          </div>
          <span class="shrink-0 font-code text-[11px] tabular-nums text-muted-foreground">{{
            formatMediaTime(duration)
          }}</span>
          <button
            type="button"
            class="inline-flex size-6 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            :aria-label="
              t(isMuted ? 'mediaUi.directMedia.unmuteMedia' : 'mediaUi.directMedia.muteMedia', {
                type: label,
              })
            "
            @click="toggleMute"
          >
            <AppIcon
              :name="isMuted ? 'volume-off' : 'volume'"
              :size="14"
            />
          </button>
        </div>
      </div>
    </template>
    <span
      v-if="source && asset.kind !== 'audio'"
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
      :label="description || t('mediaUi.directMedia.linkedImage')"
      :source="original"
      @close="expanded = false"
    />
  </div>
</template>
