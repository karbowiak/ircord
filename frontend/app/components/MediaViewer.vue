<script setup lang="ts">
import type { components } from '../../shared/generated/api'
import { publicMediaURL } from '../../shared/utils/link-media'
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
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
const item = computed(() => props.items[index.value])
const many = computed(() => props.items.length > 1)
const failed = reactive(new Set<string>())
const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
onMounted(() => {
  dialog.value?.showModal()
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
onBeforeUnmount(() => {
  if (dialog.value?.open) dialog.value.close()
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
}
/** Clicks on the dark surround close; clicks on the media or controls don't. */
function surround(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (target === dialog.value || target.classList.contains('media-viewer-stage'))
    dialog.value?.close()
}
let swipe: { x: number; y: number; id: number } | undefined
function down(event: PointerEvent) {
  if (event.pointerType !== 'mouse')
    swipe = { x: event.clientX, y: event.clientY, id: event.pointerId }
}
function up(event: PointerEvent) {
  if (!swipe || swipe.id !== event.pointerId) return
  const dx = event.clientX - swipe.x
  const dy = event.clientY - swipe.y
  swipe = undefined
  if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) go(dx < 0 ? 1 : -1)
  else if (dy > 90 && Math.abs(dy) > Math.abs(dx) * 1.5) dialog.value?.close()
}
</script>
<template>
  <Teleport to="body">
    <dialog
      ref="dialog"
      class="media-viewer fixed inset-0 w-[100vw] max-w-[none] h-[100dvh] max-h-[none] m-0 p-0 border-0 grid [grid-template-rows:auto_minmax(0,_1fr)_auto] text-[#f4f5f7] [background:rgb(6_7_10_/_94%)] overscroll-contain"
      :aria-labelledby="titleId"
      @close="emit('close')"
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
          <h2 :id="titleId">{{ heading || label }}</h2>
          <p v-if="detail">{{ detail }}</p>
        </div>
        <span
          v-if="many"
          class="media-viewer-count py-[3px] px-2.5 rounded-[999px] [background:rgb(255_255_255_/_10%)] text-[12px] font-[600] tabular-nums"
          role="status"
          >{{ index + 1 }} / {{ items.length }}</span
        >
        <a
          v-if="source"
          class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap"
          :href="source"
          target="_blank"
          rel="noopener noreferrer"
          >{{ sourceLabel || 'Open original'
          }}<AppIcon
            name="external"
            :size="14"
        /></a>
        <a
          v-if="item"
          class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
          :href="publicMediaURL(item.url)"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Open media file"
          title="Open media file"
          ><AppIcon
            name="image"
            :size="16"
        /></a>
        <button
          type="button"
          class="media-viewer-action inline-flex items-center justify-center gap-1.5 min-h-9 py-0 px-3 rounded-[999px] text-inherit text-[13px] whitespace-nowrap is-icon"
          aria-label="Close viewer"
          @click="dialog?.close()"
        >
          <AppIcon
            name="close"
            :size="20"
          />
        </button>
      </header>
      <div
        class="media-viewer-stage relative flex items-center justify-center min-h-0 overflow-hidden pt-0 pr-18 pb-3 pl-18"
      >
        <template v-if="item">
          <div
            v-if="failed.has(item.url)"
            class="media-viewer-failed grid [justify-items:center] gap-2.5 text-[rgb(244_245_247_/_70%)] text-[14px]"
            role="status"
          >
            <AppIcon
              name="image"
              :size="28"
            />This media couldn’t be loaded.
          </div>
          <img
            v-else-if="item.kind === 'image'"
            :key="item.url"
            class="media-viewer-media block min-w-0 min-h-0 max-w-full max-h-full w-auto h-auto object-contain rounded-[6px] shadow-[0_12px_48px_rgb(0_0_0_/_50%)]"
            :src="item.url"
            :alt="label"
            :width="item.width || undefined"
            :height="item.height || undefined"
            referrerpolicy="no-referrer"
            @error="failed.add(item.url)"
          />
          <video
            v-else-if="item.kind === 'video'"
            :key="item.url"
            class="media-viewer-media block min-w-0 min-h-0 max-w-full max-h-full w-auto h-auto object-contain rounded-[6px] shadow-[0_12px_48px_rgb(0_0_0_/_50%)]"
            :src="item.url"
            controls
            autoplay
            playsinline
            :aria-label="label"
            @error="failed.add(item.url)"
          />
        </template>
        <button
          v-if="many && index > 0"
          type="button"
          class="media-viewer-nav absolute top-1/2 grid [place-items:center] w-12 h-12 mt-[-30px] rounded-full text-[#fff] [background:rgb(255_255_255_/_10%)] [transition:background_0.15s_ease] is-previous"
          aria-label="Previous image"
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
          aria-label="Next image"
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
        aria-label="Media in this post"
      >
        <button
          v-for="(entry, i) in items"
          :key="entry.url"
          type="button"
          class="media-viewer-thumb flex-none grid [place-items:center] w-12 h-12 overflow-hidden rounded-[8px] text-[#fff] [background:rgb(255_255_255_/_8%)] opacity-50 [outline:2px_solid_transparent] [outline-offset:1px] [transition:opacity_0.15s_ease,_outline-color_0.15s_ease]"
          :class="{ 'is-current': i === index }"
          :aria-label="`Show item ${i + 1} of ${items.length}`"
          :aria-current="i === index || undefined"
          @click="index = i"
        >
          <img
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
    </dialog>
  </Teleport>
</template>
