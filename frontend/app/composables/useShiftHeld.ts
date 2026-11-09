const sharedShiftHeld = createSharedComposable(() => {
  const held = ref(false)
  const key = (event: KeyboardEvent) =>
    (held.value = event.key === 'Shift' ? event.type === 'keydown' : event.shiftKey)
  const reset = () => (held.value = false)
  useEventListener('keydown', key, { capture: true })
  useEventListener('keyup', key, { capture: true })
  useEventListener('blur', reset)
  const visibility = useDocumentVisibility()
  watch(visibility, (state) => state === 'hidden' && reset())
  return held
})

/**
 * Whether Shift is held right now, for the message toolbar's alternate actions. Tracked
 * from key events at the window, so it doesn't matter whether Shift went down before or
 * after the pointer reached a message. Losing focus resets it: the key-up can happen in
 * another window, and the toolbar must never stay stuck in Shift mode.
 */
export function useShiftHeld() {
  return sharedShiftHeld()
}
