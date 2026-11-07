/**
 * When a GIF or video in a message may play. With autoplay on (an account preference, off by
 * default) it plays by itself; otherwise it shows a still frame and plays while the mouse is over
 * it, while it has keyboard focus, or after a first tap on touch screens. Reduced motion keeps it
 * still either way; opening the media in the viewer is then the way to watch it.
 */
export function useMediaPlayback() {
  const preferences = usePreferencesStore()
  const appearance = useAppearanceStore()
  const previewing = ref(false)
  let touch = false
  let frame: Element | null = null

  const autoplay = computed(
    () => preferences.preferences.autoplayMedia && !appearance.reducedMotion,
  )
  const playing = computed(() => autoplay.value || (previewing.value && !appearance.reducedMotion))

  /** A tap preview ends on the next touch elsewhere: not every browser focuses a tapped button. */
  function outside(event: PointerEvent) {
    if (frame && event.target instanceof Node && frame.contains(event.target)) return
    stopPreview()
  }
  function stopPreview() {
    previewing.value = false
    document.removeEventListener('pointerdown', outside, true)
  }
  onScopeDispose(stopPreview)

  /** Spread onto the element that frames the media with `v-on`. */
  const handlers = {
    pointerdown: (event: PointerEvent) => {
      touch = event.pointerType !== 'mouse'
      frame = event.currentTarget as Element
    },
    pointerenter: (event: PointerEvent) => {
      if (event.pointerType === 'mouse') previewing.value = true
    },
    pointerleave: (event: PointerEvent) => {
      if (event.pointerType === 'mouse') stopPreview()
    },
    focusin: (event: FocusEvent) => {
      if (event.target instanceof Element && event.target.matches(':focus-visible'))
        previewing.value = true
    },
    focusout: stopPreview,
  }

  /**
   * On a touch screen the first tap plays still media instead of opening it. Returns true when
   * the tap was used for that, so the caller skips its own action.
   */
  function claimTap() {
    const claim = touch && !playing.value && !appearance.reducedMotion
    touch = false
    if (claim) {
      previewing.value = true
      document.addEventListener('pointerdown', outside, true)
    }
    return claim
  }

  return { autoplay, playing, previewing, handlers, claimTap }
}
