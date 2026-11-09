<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { publicMediaURL } from '../../shared/utils/link-media'
import { offlineStatus } from '../utils/offline-archive'
/**
 * Full-screen viewer for linked photos and videos: the media as large as the screen allows,
 * paging by arrows, keys or swipe, and the post it came from one click away.
 */
const props = defineProps<{
  items: components['schemas']['LinkMediaAsset'][]
  label: string
  heading?: string
  detail?: string
  source?: string
  sourceLabel?: string
}>()
const index = defineModel<number>('index', { default: 0 })
const emit = defineEmits<{ close: [] }>()
const { t, n } = useI18n()
const zoomHelpId = useId()
const open = ref(true)
function close() {
  if (!open.value) return
  open.value = false
  emit('close')
}
const unregister = useDialogStack().register(close)
const item = computed(() => props.items[index.value])
const { source: fileSource, loading } = useOfflineMedia(() => publicMediaURL(item.value?.url ?? ''))
const many = computed(() => props.items.length > 1)
const failed = reactive(new Set<string>())
const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
onMounted(() => {
  // Neighbours are fetched ahead so paging shows the next photo at once.
  watch(
    index,
    (i) => {
      for (const near of [props.items[i - 1], props.items[i + 1]])
        if (near?.kind === 'image') {
          const image = new Image()
          image.referrerPolicy = 'no-referrer'
          image.src = near.url
        }
    },
    { immediate: true },
  )
})
// After unmount, once the focus trap has gone; while it is mounted it would pull focus back in.
onUnmounted(() => {
  unregister()
  if (opener?.isConnected) opener.focus({ preventScroll: true })
})
function go(by: number) {
  index.value = Math.max(0, Math.min(props.items.length - 1, index.value + by))
}
function keys(event: KeyboardEvent) {
  if (event.target instanceof HTMLVideoElement) return
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    go(-1)
  }
  if (event.key === 'ArrowRight') {
    event.preventDefault()
    go(1)
  }
  if (!zoomable.value || event.ctrlKey || event.metaKey || event.altKey) return
  const factor = { '+': 1.5, '=': 1.5, '-': 1 / 1.5 }[event.key]
  if (factor) {
    event.preventDefault()
    zoomBy(factor)
  } else if (event.key === '0') {
    event.preventDefault()
    resetZoom()
  }
}
/** Clicks on the dark surround close; clicks on the media or controls don't. */
function surround(event: MouseEvent) {
  const target = event.target as HTMLElement
  // The end of a pan or pinch is not a click.
  // Clicking a zoomed photo or its surround leaves the viewer open; zoom out first to close.
  if (moved || zoomed.value) return
  if (target === event.currentTarget || target.classList.contains('media-viewer-stage')) close()
}
/*
 * Zoom for photos: pinch (touch, or a trackpad pinch, which browsers report as ctrl+wheel),
 * double-tap or double-click, and + / - / 0 on the keyboard. While zoomed, dragging pans.
 * The stage takes over touch gestures for itself, so a pinch on a photo zooms the photo and
 * never the page; everywhere else the page keeps the browser's own zoom.
 */
const MAX_ZOOM = 8
const stage = ref<HTMLElement>()
const media = ref<HTMLImageElement>()
const zoom = reactive({ scale: 1, x: 0, y: 0 })
const zoomed = computed(() => zoom.scale > 1.01)
const zoomable = computed(() => item.value?.kind === 'image' && !failed.has(item.value.url))
const mediaStyle = computed(() =>
  zoom.scale === 1 && zoom.x === 0 && zoom.y === 0
    ? undefined
    : { transform: `translate(${zoom.x}px, ${zoom.y}px) scale(${zoom.scale})` },
)
function resetZoom() {
  zoom.scale = 1
  zoom.x = 0
  zoom.y = 0
}
watch(index, resetZoom)
/** Keeps a zoomed photo from being dragged off the stage. */
function clampPan() {
  const image = media.value
  const box = stage.value?.getBoundingClientRect()
  if (!image || !box) return
  const limitX = Math.max(0, (image.offsetWidth * zoom.scale - box.width) / 2)
  const limitY = Math.max(0, (image.offsetHeight * zoom.scale - box.height) / 2)
  zoom.x = Math.max(-limitX, Math.min(limitX, zoom.x))
  zoom.y = Math.max(-limitY, Math.min(limitY, zoom.y))
}
/** Sets the scale, keeping the photo point under (fx, fy) in place. */
function zoomAt(scale: number, fx: number, fy: number, from = { ...zoom }) {
  const image = media.value
  if (!image) return
  const next = Math.max(1, Math.min(MAX_ZOOM, scale))
  if (next === 1) return resetZoom()
  const box = image.getBoundingClientRect()
  // The photo's untransformed centre; scaling about the centre leaves it at box centre - pan.
  const cx = box.left + box.width / 2 - zoom.x
  const cy = box.top + box.height / 2 - zoom.y
  zoom.x = fx - cx - (next * (fx - cx - from.x)) / from.scale
  zoom.y = fy - cy - (next * (fy - cy - from.y)) / from.scale
  zoom.scale = next
  clampPan()
}
function zoomBy(factor: number) {
  const box = media.value?.getBoundingClientRect()
  if (box) zoomAt(zoom.scale * factor, box.left + box.width / 2, box.top + box.height / 2)
}
function toggleZoom(fx: number, fy: number) {
  if (zoomed.value) resetZoom()
  else zoomAt(2.5, fx, fy)
}
function wheel(event: WheelEvent) {
  if (!zoomable.value || !event.ctrlKey) return
  event.preventDefault()
  // Firefox reports a Ctrl+mouse wheel in lines rather than pixels.
  const dy = event.deltaMode === 1 ? event.deltaY * 33 : event.deltaY
  zoomAt(zoom.scale * Math.exp(-dy * 0.01), event.clientX, event.clientY)
}
/**
 * Safari's own pinch events. On a Mac trackpad they are the only report of a pinch, so they
 * drive the zoom; on touch screens the pointer handlers zoom, and these are only cancelled.
 */
type SafariGesture = Event & { scale: number; clientX: number; clientY: number }
let safariStart: { scale: number; x: number; y: number } | undefined
function gesture(event: Event) {
  if (!zoomable.value) return
  event.preventDefault()
  const pinch = event as SafariGesture
  if (event.type === 'gesturestart') safariStart = { ...zoom }
  else if (safariStart && !pointers.size && typeof pinch.scale === 'number')
    zoomAt(safariStart.scale * pinch.scale, pinch.clientX, pinch.clientY, safariStart)
}
// The stage mounts with the dialog's content, after this component, so listen once it exists.
watch(stage, (current, previous) => {
  for (const [target, add] of [
    [previous, false],
    [current, true],
  ] as const) {
    if (!target) continue
    const method = add ? 'addEventListener' : 'removeEventListener'
    target[method]('wheel', wheel as EventListener, { passive: false })
    target[method]('gesturestart', gesture, { passive: false })
    target[method]('gesturechange', gesture, { passive: false })
  }
})
const pointers = new Map<number, { x: number; y: number }>()
let gestureStart:
  { scale: number; x: number; y: number; px: number; py: number; distance: number } | undefined
let moved = false
let lastTap: { at: number; x: number; y: number } | undefined
function pointerState() {
  const points = [...pointers.values()]
  const px = points.reduce((sum, p) => sum + p.x, 0) / points.length
  const py = points.reduce((sum, p) => sum + p.y, 0) / points.length
  const distance =
    points.length > 1 ? Math.hypot(points[0]!.x - points[1]!.x, points[0]!.y - points[1]!.y) : 0
  return { px, py, distance }
}
function startGesture() {
  gestureStart = pointers.size ? { ...zoom, ...pointerState() } : undefined
}
let lastPointerType = 'mouse'
function stageDown(event: PointerEvent) {
  lastPointerType = event.pointerType
  if (!zoomable.value || (event.pointerType === 'mouse' && event.button !== 0)) return
  // Buttons and links on the stage, such as previous and next, keep their own clicks.
  if ((event.target as HTMLElement).closest('button, a, video')) return
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  // A pinch, or a drag of a zoomed photo, is not a swipe.
  if (pointers.size > 1 || zoomed.value) swipe = undefined
  startGesture()
}
function stageMove(event: PointerEvent) {
  if (!pointers.has(event.pointerId) || !gestureStart) return
  pointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
  const now = pointerState()
  const start = gestureStart
  if (Math.hypot(now.px - start.px, now.py - start.py) > 6) moved = true
  // Capture only once a drag or pinch is under way: while captured, the browser sends the
  // pointer's click to the stage instead of what is under it.
  if ((moved || pointers.size > 1) && (zoomed.value || pointers.size > 1))
    for (const id of pointers.keys())
      if (!stage.value?.hasPointerCapture?.(id)) stage.value?.setPointerCapture?.(id)
  if (pointers.size > 1 && start.distance > 0) {
    moved = true
    const scale = Math.max(1, Math.min(MAX_ZOOM, (start.scale * now.distance) / start.distance))
    if (scale === 1) return resetZoom()
    // Zoom about where the pinch started, then follow the fingers as they move.
    zoomAt(scale, start.px, start.py, start)
    zoom.x += now.px - start.px
    zoom.y += now.py - start.py
    clampPan()
  } else if (zoomed.value) {
    zoom.x = start.x + now.px - start.px
    zoom.y = start.y + now.py - start.py
    clampPan()
  }
}
function stageUp(event: PointerEvent) {
  if (!pointers.delete(event.pointerId)) return
  startGesture()
  if (pointers.size || moved || event.type === 'pointercancel') return
  if (event.pointerType === 'mouse') return // Mice use dblclick.
  const at = performance.now()
  if (
    lastTap &&
    at - lastTap.at < 320 &&
    Math.hypot(event.clientX - lastTap.x, event.clientY - lastTap.y) < 40
  ) {
    lastTap = undefined
    swipe = undefined
    // The tap's click that follows must not close the viewer.
    moved = true
    toggleZoom(event.clientX, event.clientY)
  } else lastTap = { at, x: event.clientX, y: event.clientY }
}
function doubleClick(event: MouseEvent) {
  // Touch double-taps are handled on pointerup; browsers also report them as dblclick.
  if (lastPointerType !== 'mouse') return
  const target = event.target
  if (zoomable.value && (target === media.value || (zoomed.value && target === stage.value)))
    toggleZoom(event.clientX, event.clientY)
}
let swipe: { x: number; y: number; id: number } | undefined
function down(event: PointerEvent) {
  if (pointers.size <= 1) moved = false
  if (event.pointerType !== 'mouse' && !zoomed.value && pointers.size < 2)
    swipe = { x: event.clientX, y: event.clientY, id: event.pointerId }
}
function up(event: PointerEvent) {
  if (!swipe || swipe.id !== event.pointerId) return
  const dx = event.clientX - swipe.x
  const dy = event.clientY - swipe.y
  swipe = undefined
  if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) go(dx < 0 ? 1 : -1)
  else if (dy > 90 && Math.abs(dy) > Math.abs(dx) * 1.5) close()
}
</script>
<template>
  <UiDialog
    :open="open"
    @update:open="(value) => value || close()"
  >
    <UiDialogContent
      class="media-viewer w-[100vw] max-w-[none] h-[100dvh] max-h-[none] m-0 rounded-none shadow-none grid [grid-template-rows:auto_minmax(0,_1fr)_auto] text-[#f4f5f7] bg-[rgb(6_7_10_/_94%)] overscroll-contain"
      overlay-class="bg-[rgb(6_7_10_/_60%)] backdrop-blur-[6px]"
      :aria-describedby="undefined"
      @close-auto-focus.prevent
      @click="surround"
      @keydown="keys"
      @pointerdown="down"
      @pointerup="up"
      @pointercancel="swipe = undefined"
    >
      <header
        class="media-viewer-bar flex items-center gap-2 min-w-0 pt-[max(10px,_env(safe-area-inset-top))] pr-3 pb-2.5 pl-5"
      >
        <div class="media-viewer-heading flex-1 min-w-0">
          <UiDialogTitle as="h2">{{ heading || label }}</UiDialogTitle>
          <p v-if="detail">{{ detail }}</p>
        </div>
        <span
          v-if="many"
          class="media-viewer-count py-[3px] px-2.5 rounded-[999px] [background:rgb(255_255_255_/_10%)] text-[12px] font-[600] tabular-nums"
          role="status"
          >{{ n(index + 1) }} / {{ n(items.length) }}</span
        >
        <a
          v-if="source && !offlineStatus.offline"
          class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap"
          :href="source"
          target="_blank"
          rel="noopener noreferrer"
          >{{ sourceLabel || $t('mediaUi.viewer.openOriginal')
          }}<AppIcon
            name="external"
            :size="14"
        /></a>
        <a
          v-if="item && fileSource"
          class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
          :href="fileSource"
          :download="offlineStatus.offline ? heading || 'image' : undefined"
          target="_blank"
          rel="noopener noreferrer"
          :aria-label="$t('mediaUi.viewer.openMediaFile')"
          :title="$t('mediaUi.viewer.openMediaFile')"
          ><AppIcon
            name="image"
            :size="16"
        /></a>
        <button
          type="button"
          class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
          :aria-label="$t('mediaUi.viewer.close')"
          @click="close"
        >
          <AppIcon
            name="close"
            :size="20"
          />
        </button>
      </header>
      <div
        ref="stage"
        class="media-viewer-stage relative flex items-center justify-center min-h-0 overflow-hidden pt-0 pr-18 pb-3 pl-18"
        :class="{ 'is-zoomable': zoomable, 'is-zoomed': zoomed }"
        @pointerdown="stageDown"
        @pointermove="stageMove"
        @pointerup="stageUp"
        @pointercancel="stageUp"
        @dblclick="doubleClick"
      >
        <template v-if="item">
          <div
            v-if="failed.has(item.url) || (offlineStatus.offline && !fileSource && !loading)"
            class="media-viewer-failed grid [justify-items:center] gap-2.5 text-[rgb(244_245_247_/_70%)] text-[14px]"
            role="status"
          >
            <AppIcon
              name="image"
              :size="28"
            />{{
              offlineStatus.offline && !fileSource
                ? $t('mediaUi.viewer.failedOffline')
                : $t('mediaUi.viewer.loadFailed')
            }}
          </div>
          <!-- Native on purpose: the source may be a blob: URL for a saved offline copy. -->
          <img
            v-else-if="item.kind === 'image' && fileSource"
            :key="item.url"
            ref="media"
            class="media-viewer-media block min-w-0 min-h-0 max-w-full max-h-full w-auto h-auto object-contain rounded-[6px] shadow-[0_12px_48px_rgb(0_0_0_/_50%)]"
            :src="fileSource"
            :alt="label"
            :width="item.width || undefined"
            :height="item.height || undefined"
            :style="mediaStyle"
            draggable="false"
            referrerpolicy="no-referrer"
            @error="failed.add(item.url)"
          />
          <video
            v-else-if="item.kind === 'video'"
            :key="item.url"
            class="media-viewer-media block min-w-0 min-h-0 max-w-full max-h-full w-auto h-auto object-contain rounded-[6px] shadow-[0_12px_48px_rgb(0_0_0_/_50%)]"
            :src="fileSource"
            controls
            autoplay
            playsinline
            :aria-label="label"
            @error="failed.add(item.url)"
          />
        </template>
        <div
          v-if="zoomable && fileSource"
          class="absolute top-2 right-2 z-10 inline-flex items-center gap-1 rounded-full p-1 [background:rgb(6_7_10_/_70%)]"
          role="group"
          :aria-label="t('mediaUi.viewer.zoomControls')"
        >
          <button
            type="button"
            class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
            :aria-label="t('mediaUi.viewer.zoomOut')"
            :title="t('mediaUi.viewer.zoomOut')"
            :aria-describedby="zoomHelpId"
            :disabled="zoom.scale <= 1"
            @click="zoomBy(1 / 1.5)"
          >
            <span aria-hidden="true">−</span>
          </button>
          <button
            type="button"
            class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
            :aria-label="t('mediaUi.viewer.zoomIn')"
            :title="t('mediaUi.viewer.zoomIn')"
            :aria-describedby="zoomHelpId"
            :disabled="zoom.scale >= MAX_ZOOM"
            @click="zoomBy(1.5)"
          >
            <span aria-hidden="true">+</span>
          </button>
          <button
            type="button"
            class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
            :aria-label="t('mediaUi.viewer.resetZoom')"
            :title="t('mediaUi.viewer.resetZoom')"
            :aria-describedby="zoomHelpId"
            :disabled="!zoomed"
            @click="resetZoom"
          >
            <AppIcon
              name="reset"
              :size="15"
            />
          </button>
          <span
            :id="zoomHelpId"
            class="sr-only"
            >{{ $t('mediaUi.viewer.zoomHelp') }}</span
          >
        </div>
        <button
          v-if="many && index > 0"
          type="button"
          class="media-viewer-nav absolute top-1/2 grid [place-items:center] w-12 h-12 mt-[-30px] rounded-full text-[#fff] [background:rgb(255_255_255_/_10%)] [transition:background_0.15s_ease] is-previous"
          :aria-label="$t('mediaUi.viewer.previousImage')"
          @click="go(-1)"
        >
          <AppIcon
            name="left"
            :size="24"
          />
        </button>
        <button
          v-if="many && index < items.length - 1"
          type="button"
          class="media-viewer-nav absolute top-1/2 grid [place-items:center] w-12 h-12 mt-[-30px] rounded-full text-[#fff] [background:rgb(255_255_255_/_10%)] [transition:background_0.15s_ease] is-next"
          :aria-label="$t('mediaUi.viewer.nextImage')"
          @click="go(1)"
        >
          <AppIcon
            name="right"
            :size="24"
          />
        </button>
      </div>
      <div
        v-if="many && items.length <= 20"
        class="media-viewer-strip flex [justify-content:safe_center] gap-1.5 overflow-x-auto pt-1 pr-4 pb-[max(14px,_env(safe-area-inset-bottom))] pl-4 [scrollbar-width:none]"
        :aria-label="$t('mediaUi.viewer.mediaInPost')"
      >
        <button
          v-for="(entry, i) in items"
          :key="entry.url"
          type="button"
          class="media-viewer-thumb flex-none grid [place-items:center] w-12 h-12 overflow-hidden rounded-[8px] text-[#fff] [background:rgb(255_255_255_/_8%)] opacity-50 [outline:2px_solid_transparent] [outline-offset:1px] [transition:opacity_0.15s_ease,_outline-color_0.15s_ease]"
          :class="{ 'is-current': i === index }"
          :aria-label="
            $t('mediaUi.viewer.showItem', {
              current: n(i + 1),
              total: n(items.length),
            })
          "
          :aria-current="i === index || undefined"
          @click="index = i"
        >
          <NuxtImg
            v-if="entry.kind === 'image'"
            :src="entry.url"
            alt=""
            loading="lazy"
            referrerpolicy="no-referrer"
          />
          <AppIcon
            v-else
            name="play"
            :size="16"
          />
        </button>
      </div>
    </UiDialogContent>
  </UiDialog>
</template>
