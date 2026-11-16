import { computed, shallowRef, toValue, type MaybeRefOrGetter } from 'vue'

const MEDIA_GROUP = '.group\\/media'

/** The element under the latest touch, shared by every embed on the page. */
const touched = shallowRef<Element | null>(null)
let listening = false

function listen() {
  if (listening || typeof document === 'undefined') return
  listening = true
  document.addEventListener(
    'pointerdown',
    (event) => {
      if (event.pointerType === 'mouse') return
      touched.value = event.target instanceof Element ? event.target : null
    },
    { capture: true, passive: true },
  )
}

/**
 * Touch screens have no hover, so an embed's own buttons stay hidden until the embed is
 * tapped, and hide again when the next tap lands elsewhere. Mouse hover and keyboard focus
 * are handled in CSS by the `group/media` container.
 */
export function useTouchMediaReveal(el: MaybeRefOrGetter<Element | null | undefined>) {
  listen()
  return computed(() => {
    const target = touched.value
    let group = toValue(el)?.closest(MEDIA_GROUP)
    while (target && group) {
      if (group.contains(target)) return true
      group = group.parentElement?.closest(MEDIA_GROUP)
    }
    return false
  })
}
