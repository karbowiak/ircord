import { onScopeDispose, ref, toValue, watch } from 'vue'
import type { MaybeRefOrGetter, Ref } from 'vue'

/** Muted previews may follow hover; native controls own playback after explicit interaction. */
export function useInlinePlayback(
  player: Ref<HTMLMediaElement | null>,
  active: MaybeRefOrGetter<boolean>,
  preview: MaybeRefOrGetter<boolean>,
  unavailable: (error: unknown) => void,
) {
  const playing = ref(false)
  let manual = false
  let generation = 0
  function pause() {
    generation++
    if (player.value && !player.value.paused) player.value.pause()
    playing.value = false
  }
  function takeControl() {
    manual = true
  }
  function volumeChanged() {
    // Native mute controls can move focus/pointer into browser-owned UI. Do not remute or
    // pause audible playback when that happens, even if no pointer event reaches the page.
    if (player.value && !player.value.muted) takeControl()
  }
  function sync() {
    const media = player.value
    if (!media) return
    if (!toValue(active)) return pause()
    if (manual) return
    if (!toValue(preview)) return pause()
    if (!media.muted) return takeControl()
    if (!media.paused) return
    const requested = ++generation
    void media.play().catch((error: unknown) => {
      if (requested !== generation || media !== player.value) return
      const name = error && typeof error === 'object' && 'name' in error ? error.name : undefined
      if (name !== 'AbortError' && name !== 'NotAllowedError') unavailable(error)
    })
  }
  function played() {
    if (!toValue(active)) return pause()
    playing.value = true
    if (!toValue(preview) || !player.value?.muted) takeControl()
  }
  function paused() {
    playing.value = false
    // This is observed state, never a command to start the hover controller again.
  }
  function reset() {
    pause()
    manual = false
  }
  watch([() => toValue(active), () => toValue(preview), player], sync, { flush: 'post' })
  onScopeDispose(pause)
  return { playing, takeControl, volumeChanged, played, paused, pause, reset, sync }
}
