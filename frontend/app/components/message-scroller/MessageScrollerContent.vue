<script setup vapor lang="ts">
// Vendored from shadcn-vue’s MessageScroller; styling lives with the timeline's own CSS.
import { onBeforeUnmount, onMounted, useTemplateRef } from 'vue'
import { useMessageScrollerContext } from '../../composables/useMessageScroller'

const { handleContentChange, handleResize, setContentElement, setSpacerElement } =
  useMessageScrollerContext()

const contentRef = useTemplateRef<HTMLElement>('content')
const spacerRef = useTemplateRef<HTMLElement>('spacer')

let mutationObserver: MutationObserver | null = null
let resizeObserver: ResizeObserver | null = null
let resizeFrame = 0

onMounted(() => {
  const content = contentRef.value
  if (!content) return

  setContentElement(content)
  setSpacerElement(spacerRef.value ?? null)
  handleContentChange()

  if (typeof MutationObserver !== 'undefined') {
    mutationObserver = new MutationObserver(() => handleContentChange())
    mutationObserver.observe(content, { childList: true })
  }

  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => {
      window.cancelAnimationFrame(resizeFrame)
      resizeFrame = window.requestAnimationFrame(handleResize)
    })
    resizeObserver.observe(content)
  }
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(resizeFrame)
  mutationObserver?.disconnect()
  resizeObserver?.disconnect()
  mutationObserver = null
  resizeObserver = null
  setContentElement(null)
  setSpacerElement(null)
})
</script>

<template>
  <div
    ref="content"
    data-slot="message-scroller-content"
  >
    <slot />
    <div
      ref="spacer"
      aria-hidden="true"
      data-message-scroller-spacer=""
      hidden
    />
  </div>
</template>
