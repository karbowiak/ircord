<script setup lang="ts">
/**
 * Drag handle for a panel edge. `direction` says which way the pointer moves to grow the panel.
 * Arrow keys resize by 16px, Home/End jump to the limits and double-click restores the default.
 * `scale` is how many screen pixels one unit of `value` takes (a panel sized with the interface
 * scale), so the edge stays under the pointer while dragging.
 */
const props = defineProps<{
  label: string
  value: number
  min: number
  max: number
  default: number
  direction: 'right' | 'left' | 'up-left'
  scale?: number
}>()
const emit = defineEmits<{ resize: [value: number]; resizeY: [delta: number]; end: [] }>()
const dragging = ref(false)
let start = { x: 0, y: 0, value: 0 }

function down(event: PointerEvent) {
  if (event.button !== 0) return
  event.preventDefault()
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  start = { x: event.clientX, y: event.clientY, value: props.value }
  dragging.value = true
  document.documentElement.classList.add('is-resizing')
}
function move(event: PointerEvent) {
  if (!dragging.value) return
  const dx = (event.clientX - start.x) / (props.scale ?? 1)
  emit('resize', start.value + (props.direction === 'right' ? dx : -dx))
  if (props.direction === 'up-left') emit('resizeY', start.y - event.clientY)
}
function up() {
  if (!dragging.value) return
  dragging.value = false
  document.documentElement.classList.remove('is-resizing')
  emit('end')
}
function key(event: KeyboardEvent) {
  const grow = props.direction === 'right' ? 'ArrowRight' : 'ArrowLeft'
  const shrink = props.direction === 'right' ? 'ArrowLeft' : 'ArrowRight'
  const next =
    event.key === grow
      ? props.value + 16
      : event.key === shrink
        ? props.value - 16
        : event.key === 'Home'
          ? props.min
          : event.key === 'End'
            ? props.max
            : undefined
  if (next === undefined) return
  event.preventDefault()
  emit('resize', next)
}
</script>
<template>
  <div
    class="resize-handle absolute top-0 bottom-0 w-1.5 z-15 cursor-col-resize [touch-action:none] before:absolute before:inset-y-0 before:left-0.5 before:w-0.5 before:bg-primary before:opacity-0 before:content-[''] before:transition-opacity before:duration-150 before:delay-100 hover:before:opacity-70 focus-visible:before:opacity-70 focus-visible:outline-none! [&.is-dragging]:before:opacity-70 [&.resize-right]:-right-[3px] [&.resize-left]:-left-[3px] [@media(pointer:coarse)]:w-4 [@media(pointer:coarse)]:[&.resize-right]:-right-2 [@media(pointer:coarse)]:[&.resize-left]:-left-2"
    :class="[`resize-${direction}`, { 'is-dragging': dragging }]"
    role="separator"
    :aria-orientation="direction === 'up-left' ? undefined : 'vertical'"
    :aria-label="label"
    :aria-valuenow="value"
    :aria-valuemin="min"
    :aria-valuemax="max"
    tabindex="0"
    :title="$t('workspaceUi.controls.resizeReset', { label })"
    @pointerdown="down"
    @pointermove="move"
    @pointerup="up"
    @pointercancel="up"
    @lostpointercapture="up"
    @dblclick="emit('resize', props.default)"
    @keydown="key"
  />
</template>
