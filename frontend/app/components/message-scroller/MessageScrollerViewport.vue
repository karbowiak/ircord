<script setup vapor lang="ts">
// Vendored from shadcn-vue’s MessageScroller; styling lives with the timeline's own CSS.
import { onBeforeUnmount, onMounted, useTemplateRef, watch } from 'vue'
import { SCROLL_KEYS, useMessageScrollerContext } from '../../composables/useMessageScroller'

const props = withDefaults(defineProps<{ preserveScrollOnPrepend?: boolean }>(), {
  preserveScrollOnPrepend: true,
})

const {
  autoscrolling,
  handleResize,
  scrollableAttr,
  setPreserveScrollOnPrepend,
  setViewportElement,
  syncAfterScroll,
  userPointer,
  userScrollIntent,
} = useMessageScrollerContext()

// A held finger or mouse button (dragging the scrollbar, selecting text) holds automatic
// scrolling until released anywhere. Touch uses touch events: pointers cancel once a pan starts.
function onPointerDown(event: PointerEvent) {
  if (event.pointerType !== 'touch') userPointer(true)
}
const release = () => userPointer(false)
useEventListener(window, 'pointerup', release, { passive: true })
useEventListener(window, 'pointercancel', (event: PointerEvent) => {
  if (event.pointerType !== 'touch') release()
})
useEventListener(window, 'touchend', release, { passive: true })
useEventListener(window, 'touchcancel', release, { passive: true })
useEventListener(window, 'blur', release)

const viewportEl = useTemplateRef<HTMLElement>('viewport')
defineExpose({ element: viewportEl })

watch(() => props.preserveScrollOnPrepend, setPreserveScrollOnPrepend, { immediate: true })

function onKeyDown(event: KeyboardEvent) {
  if (SCROLL_KEYS.has(event.key)) userScrollIntent()
}

let resizeObserver: ResizeObserver | null = null
let resizeFrame = 0

onMounted(() => {
  const viewport = viewportEl.value
  setViewportElement(viewport)
  if (!viewport || typeof ResizeObserver === 'undefined') return
  resizeObserver = new ResizeObserver(() => {
    window.cancelAnimationFrame(resizeFrame)
    resizeFrame = window.requestAnimationFrame(handleResize)
  })
  resizeObserver.observe(viewport)
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(resizeFrame)
  resizeObserver?.disconnect()
  resizeObserver = null
  setViewportElement(null)
})
</script>

<template>
  <div
    ref="viewport"
    data-slot="message-scroller-viewport"
    :data-scrollable="scrollableAttr"
    :data-autoscrolling="autoscrolling ? '' : undefined"
    @scroll="syncAfterScroll()"
    @pointerdown.passive="onPointerDown"
    @touchstart.passive="userPointer(true)"
    @wheel.passive="userScrollIntent()"
    @touchmove.passive="userScrollIntent()"
    @keydown="onKeyDown"
  >
    <slot />
  </div>
</template>
