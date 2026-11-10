const held = shallowRef(false)
let readers = 0
let stop: (() => void) | undefined

// Global listeners own only the shared key state, never the first message's effect scope.
function listen() {
  const key = (event: KeyboardEvent) =>
    (held.value = event.key === 'Shift' ? event.type === 'keydown' : event.shiftKey)
  const reset = () => (held.value = false)
  const visibility = () => {
    if (document.hidden) reset()
  }
  window.addEventListener('keydown', key, { capture: true })
  window.addEventListener('keyup', key, { capture: true })
  window.addEventListener('blur', reset)
  document.addEventListener('visibilitychange', visibility)
  return () => {
    window.removeEventListener('keydown', key, { capture: true })
    window.removeEventListener('keyup', key, { capture: true })
    window.removeEventListener('blur', reset)
    document.removeEventListener('visibilitychange', visibility)
    reset()
  }
}

/**
 * Whether Shift is held right now, for the message toolbar's alternate actions. Tracked
 * from key events at the window, so it doesn't matter whether Shift went down before or
 * after the pointer reached a message. Losing focus resets it: the key-up can happen in
 * another window, and the toolbar must never stay stuck in Shift mode.
 */
export function useShiftHeld() {
  if (import.meta.server) return readonly(held)
  if (++readers === 1) stop = listen()
  onScopeDispose(() => {
    if (--readers === 0) {
      stop?.()
      stop = undefined
    }
  })
  return readonly(held)
}

if (import.meta.hot) import.meta.hot.dispose(() => stop?.())
