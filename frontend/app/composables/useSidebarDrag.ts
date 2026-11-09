/**
 * Drag to reorder rows of one conversation-list section, with a mouse, pen or finger.
 * A mouse or pen drag starts after a short move. A touch drag starts when you hold a row and
 * then move it, so a quick swipe still scrolls the list. Escape cancels a drag.
 * Rows mark themselves with `data-sort-id` and `data-sort-section`.
 */
export type DropPlace = 'before' | 'after'

const holdMs = 300
const mouseSlop = 5
const touchSlop = 8
const edge = 32

export function useSidebarDrag(
  scroller: Ref<HTMLElement | null>,
  onDrop: (section: string, id: string, target: string, place: DropPlace) => void,
) {
  const dragging = ref<{ id: string; section: string } | null>(null)
  const target = ref<{ id: string; place: DropPlace } | null>(null)
  let start: { x: number; y: number; id: string; section: string; pointerId: number } | null = null
  let touch = false
  let armed = false
  let cancelled = false
  let holdTimer: ReturnType<typeof setTimeout> | undefined
  let swallowClick = false
  let scrollFrame = 0
  let lastY = 0

  function reset() {
    clearTimeout(holdTimer)
    cancelAnimationFrame(scrollFrame)
    start = null
    armed = false
    cancelled = false
    dragging.value = null
    target.value = null
    document.documentElement.classList.remove('is-sorting')
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    window.removeEventListener('pointercancel', reset)
    window.removeEventListener('keydown', key, true)
  }

  function down(event: PointerEvent, section: string, id: string) {
    if (event.button !== 0 || start) return
    start = { x: event.clientX, y: event.clientY, id, section, pointerId: event.pointerId }
    touch = event.pointerType === 'touch'
    armed = !touch
    if (touch) holdTimer = setTimeout(() => (armed = true), holdMs)
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
    window.addEventListener('pointercancel', reset)
    window.addEventListener('keydown', key, true)
  }

  function rowAt(x: number, y: number) {
    const row = document.elementFromPoint(x, y)?.closest<HTMLElement>('[data-sort-id]')
    return row && row.dataset.sortSection === start?.section ? row : null
  }

  function autoScroll() {
    const el = scroller.value
    if (!el || !dragging.value) return
    const box = el.getBoundingClientRect()
    const step = lastY < box.top + edge ? -8 : lastY > box.bottom - edge ? 8 : 0
    if (step) el.scrollTop += step
    scrollFrame = requestAnimationFrame(autoScroll)
  }

  function move(event: PointerEvent) {
    if (!start || cancelled || event.pointerId !== start.pointerId) return
    const distance = Math.hypot(event.clientX - start.x, event.clientY - start.y)
    if (!dragging.value) {
      // A touch that moves before the hold ends is a scroll, not a drag.
      if (!armed) return distance > touchSlop ? reset() : undefined
      if (distance < (touch ? 2 : mouseSlop)) return
      dragging.value = { id: start.id, section: start.section }
      document.documentElement.classList.add('is-sorting')
      scrollFrame = requestAnimationFrame(autoScroll)
    }
    event.preventDefault()
    lastY = event.clientY
    const row = rowAt(event.clientX, event.clientY)
    if (!row) return
    const id = row.dataset.sortId!
    const box = row.getBoundingClientRect()
    const place: DropPlace = event.clientY < box.top + box.height / 2 ? 'before' : 'after'
    target.value = id === start.id ? null : { id, place }
  }

  function up(event: PointerEvent) {
    if (!start || event.pointerId !== start.pointerId) return
    const drop = dragging.value && target.value
    if (dragging.value || cancelled) {
      // The click that follows this release belongs to the drag; later clicks don't.
      swallowClick = true
      setTimeout(() => (swallowClick = false))
    }
    if (drop) onDrop(start.section, start.id, drop.id, drop.place)
    reset()
  }

  /** Escape drops nothing; the press stays ignored until the button is released. */
  function key(event: KeyboardEvent) {
    if (event.key !== 'Escape' || !dragging.value) return
    event.preventDefault()
    event.stopPropagation()
    cancelled = true
    dragging.value = null
    target.value = null
    cancelAnimationFrame(scrollFrame)
    document.documentElement.classList.remove('is-sorting')
  }

  /** The click that ends a drag must not open the dragged conversation. */
  function click(event: MouseEvent) {
    if (!swallowClick) return
    swallowClick = false
    event.preventDefault()
    event.stopPropagation()
  }
  /** Once a touch drag holds a row, the list must not scroll under the finger. */
  function touchmove(event: TouchEvent) {
    if ((armed && touch && start) || dragging.value) event.preventDefault()
  }
  /** A drag in progress must not open the row's menu; a still hold still can. */
  function contextmenu(event: MouseEvent) {
    if (dragging.value) event.preventDefault()
    else if (start) reset()
  }

  useEventListener(scroller, 'click', click, { capture: true })
  useEventListener(scroller, 'touchmove', touchmove, { passive: false })
  useEventListener(scroller, 'contextmenu', contextmenu, { capture: true })
  onScopeDispose(reset)

  return { dragging, target, down }
}
