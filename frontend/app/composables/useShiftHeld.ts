const held = ref(false)
let installed = false

/**
 * Whether Shift is held right now, for the message toolbar's alternate actions. Tracked
 * from key events at the window, so it doesn't matter whether Shift went down before or
 * after the pointer reached a message. Losing focus resets it: the key-up can happen in
 * another window, and the toolbar must never stay stuck in Shift mode.
 */
export function useShiftHeld() {
  if (!installed && typeof window !== 'undefined') {
    installed = true
    const key = (event: KeyboardEvent) =>
      (held.value = event.key === 'Shift' ? event.type === 'keydown' : event.shiftKey)
    const reset = () => (held.value = false)
    window.addEventListener('keydown', key, true)
    window.addEventListener('keyup', key, true)
    window.addEventListener('blur', reset)
    document.addEventListener('visibilitychange', () => document.hidden && reset())
  }
  return held
}
