<script setup lang="ts">
import type { ViewerItem } from '../../shared/types/chat'
/**
 * Full-size image viewer: the image centred and fitted to the window (never enlarged past its
 * own size) on a dark backdrop. Esc, the close button or clicking anywhere but the image closes
 * it; the native dialog keeps focus inside and returns it to whatever opened the viewer.
 * A gallery pages through its images with the arrow buttons, the ← and → keys, or a swipe.
 */
const props = defineProps<{ items: ViewerItem[]; start?: number }>()
const emit = defineEmits<{ close: [] }>()
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
const state = ref<'loading' | 'ready' | 'failed'>('loading')
const size = ref('')
const index = ref(Math.min(Math.max(props.start ?? 0, 0), Math.max(props.items.length - 1, 0)))
const current = computed(() => props.items[index.value] ?? { url: '', name: '' })
const url = computed(() => current.value.url)
const name = computed(() => current.value.name)
const many = computed(() => props.items.length > 1)
function go(step: number) {
  if (!many.value) return
  index.value = (index.value + step + props.items.length) % props.items.length
  state.value = 'loading'
  size.value = ''
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'ArrowLeft') go(-1)
  else if (event.key === 'ArrowRight') go(1)
  else return
  event.preventDefault()
}
/** A horizontal swipe of at least 50px pages; anything shorter is a tap. */
let swipeFrom: { x: number; y: number } | null = null
let swiped = false
function pointerdown(event: PointerEvent) {
  swiped = false
  if (event.pointerType !== 'mouse') swipeFrom = { x: event.clientX, y: event.clientY }
}
function pointerup(event: PointerEvent) {
  if (!swipeFrom) return
  const dx = event.clientX - swipeFrom.x
  const dy = event.clientY - swipeFrom.y
  swipeFrom = null
  if (Math.abs(dx) >= 50 && Math.abs(dx) > Math.abs(dy) * 1.5) {
    swiped = true
    go(dx < 0 ? 1 : -1)
  }
}
/** Same-origin files (uploads) can be saved directly; elsewhere the browser opens the link. */
const sameOrigin = computed(() => {
  try {
    return new URL(url.value, location.href).origin === location.origin
  } catch {
    return false
  }
})
function loaded(event: Event) {
  const img = event.target as HTMLImageElement
  size.value = `${img.naturalWidth} × ${img.naturalHeight}`
  state.value = 'ready'
}
/** Clicks on the image or the toolbar stay open; anything else is the backdrop. */
function stageClick(event: MouseEvent) {
  if (swiped) {
    swiped = false
    return
  }
  if (!(event.target as HTMLElement).closest('img, .image-viewer-bar, .image-viewer-nav'))
    dialog.value?.close()
}
onMounted(() => dialog.value?.showModal())
</script>
<template>
  <Teleport to="body"
    ><dialog
      ref="dialog"
      class="image-viewer"
      :aria-labelledby="titleId"
      @close="emit('close')"
      @click="stageClick"
      @keydown="keydown"
      @pointerdown="pointerdown"
      @pointerup="pointerup"
    >
      <header class="image-viewer-bar">
        <span class="image-viewer-title"
          ><strong :id="titleId">{{ name || 'Image' }}</strong
          ><small
            v-if="many"
            aria-live="polite"
            >{{ index + 1 }} of {{ items.length }}</small
          ><small v-if="size">{{ size }}</small></span
        >
        <a
          class="icon-button"
          :href="url"
          target="_blank"
          rel="noopener noreferrer"
          aria-label="Open original"
          title="Open original"
          ><AppIcon
            name="external"
            :size="18"
        /></a>
        <a
          v-if="sameOrigin"
          class="icon-button"
          :href="url"
          :download="name || ''"
          aria-label="Download"
          title="Download"
          ><AppIcon
            name="download"
            :size="18"
        /></a>
        <button
          class="icon-button"
          aria-label="Close image"
          title="Close (Esc)"
          @click="dialog?.close()"
        >
          <AppIcon
            name="close"
            :size="20"
          />
        </button>
      </header>
      <div class="image-viewer-stage">
        <span
          v-if="state === 'loading'"
          class="image-viewer-status"
          role="status"
          >Loading…</span
        >
        <p
          v-else-if="state === 'failed'"
          class="image-viewer-status"
          role="alert"
        >
          This image couldn’t be loaded.
          <a
            :href="url"
            target="_blank"
            rel="noopener noreferrer"
            >Open the original</a
          >
        </p>
        <button
          v-if="many"
          class="icon-button image-viewer-nav is-previous"
          aria-label="Previous image"
          title="Previous (←)"
          @click="go(-1)"
        >
          <AppIcon
            name="left"
            :size="28"
          />
        </button>
        <button
          v-if="many"
          class="icon-button image-viewer-nav is-next"
          aria-label="Next image"
          title="Next (→)"
          @click="go(1)"
        >
          <AppIcon
            name="right"
            :size="28"
          />
        </button>
        <img
          v-show="state === 'ready'"
          :key="url"
          :src="url"
          :alt="name"
          decoding="async"
          referrerpolicy="no-referrer"
          @load="loaded"
          @error="state = 'failed'"
        />
      </div></dialog
  ></Teleport>
</template>
