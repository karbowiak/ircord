import { isEditable } from '../../shared/utils/focus'

/**
 * Touch support for the workspace:
 * - Long-press opens the same context menus as right-click (iOS Safari never fires
 *   `contextmenu`; Android does, so the native event is used when it arrives first).
 * - Swiping in from the left edge opens navigation, from the right edge the member list.
 * - On iOS the visual viewport shrinks for the keyboard while 100dvh does not; --app-height
 *   follows the visual viewport so the composer stays above the keyboard.
 */
export function useTouchGestures(options: {
  openNav: () => void
  openMembers: () => void
  closeDrawers: () => void
  drawerOpen: () => boolean
}) {
  const LONG_PRESS = 480
  const SLOP = 10
  let timer: ReturnType<typeof setTimeout> | undefined
  let start: { x: number; y: number; target: Element; time: number } | null = null
  let longPressed = false
  let nativeMenu = false

  function cancel() {
    clearTimeout(timer)
    timer = undefined
  }
  function pointerDown(event: PointerEvent) {
    if (event.pointerType !== 'touch') return
    start = {
      x: event.clientX,
      y: event.clientY,
      target: event.target as Element,
      time: Date.now(),
    }
    longPressed = false
    nativeMenu = false
    cancel()
    // Text inputs keep their own long-press (selection, paste).
    if (isEditable(event.target)) return
    timer = setTimeout(() => {
      if (!start || nativeMenu) return
      longPressed = true
      navigator.vibrate?.(10)
      start.target.dispatchEvent(
        new MouseEvent('contextmenu', {
          bubbles: true,
          cancelable: true,
          clientX: start.x,
          clientY: start.y,
        }),
      )
    }, LONG_PRESS)
  }
  function pointerMove(event: PointerEvent) {
    if (!start || event.pointerType !== 'touch') return
    if (Math.hypot(event.clientX - start.x, event.clientY - start.y) > SLOP) cancel()
  }
  function pointerUp(event: PointerEvent) {
    if (event.pointerType !== 'touch' || !start) return
    cancel()
    const dx = event.clientX - start.x
    const dy = event.clientY - start.y
    const fast = Date.now() - start.time < 600
    // Horizontal swipes that start at a screen edge open the drawers; any swipe closes them.
    if (fast && Math.abs(dx) > 60 && Math.abs(dy) < Math.abs(dx) * 0.6) {
      if (options.drawerOpen()) options.closeDrawers()
      else if (dx > 0 && start.x < 28) options.openNav()
      else if (dx < 0 && start.x > window.innerWidth - 28) options.openMembers()
    }
    start = null
  }
  // A native long-press menu (Android) wins; ours is then skipped, and vice versa.
  function contextMenu(event: MouseEvent) {
    if (!event.isTrusted) return
    if (longPressed) {
      event.preventDefault()
      event.stopImmediatePropagation()
    } else nativeMenu = true
  }
  // The finger lifting after a long-press must not also click whatever is underneath.
  function click(event: MouseEvent) {
    if (!longPressed) return
    longPressed = false
    event.preventDefault()
    event.stopImmediatePropagation()
  }

  function viewport() {
    const vv = window.visualViewport
    if (!vv) return
    document.documentElement.style.setProperty('--app-height', `${Math.round(vv.height)}px`)
    // iOS scrolls the page to reveal the focused input; the layout already did that.
    if (window.scrollY) window.scrollTo(0, 0)
  }

  onMounted(() => {
    document.addEventListener('pointerdown', pointerDown, { passive: true })
    document.addEventListener('pointermove', pointerMove, { passive: true })
    document.addEventListener('pointerup', pointerUp, { passive: true })
    document.addEventListener('pointercancel', cancel, { passive: true })
    document.addEventListener('contextmenu', contextMenu, true)
    document.addEventListener('click', click, true)
    window.visualViewport?.addEventListener('resize', viewport)
    viewport()
  })
  onBeforeUnmount(() => {
    cancel()
    document.removeEventListener('pointerdown', pointerDown)
    document.removeEventListener('pointermove', pointerMove)
    document.removeEventListener('pointerup', pointerUp)
    document.removeEventListener('pointercancel', cancel)
    document.removeEventListener('contextmenu', contextMenu, true)
    document.removeEventListener('click', click, true)
    window.visualViewport?.removeEventListener('resize', viewport)
  })
}
