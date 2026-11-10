import { onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import type { Ref } from 'vue'

type Visibility = { preload: boolean; visible: boolean }
type Subscriber = (value: Partial<Visibility>) => void
type ObserverGroup = {
  subscribers: Map<Element, Subscriber>
  preload: IntersectionObserver
  visible: IntersectionObserver
  pending: Set<Element>
  dispose: () => void
}

// Share two observers per scroll region. Each URL has its own target, so one long
// message cannot activate all its players just because its first link is visible.
const groups = new Map<Element | null, ObserverGroup>()

function observe(element: HTMLElement, update: Subscriber) {
  const root = element.closest('.timeline-scroll, .saved-results')
  let group = groups.get(root)
  if (!group) {
    const subscribers = new Map<Element, Subscriber>()
    const pending = new Set<Element>()
    const scrollTarget = root ?? window
    let scrolling = false
    let frame: number | undefined
    let settled: ReturnType<typeof setTimeout> | undefined
    function schedule() {
      if (scrolling || frame !== undefined || !pending.size) return
      frame = requestAnimationFrame(() => {
        frame = undefined
        const target = pending.values().next().value
        if (target) {
          pending.delete(target)
          subscribers.get(target)?.({ visible: true })
        }
        // Let the browser paint the lightweight rows between player startups.
        schedule()
      })
    }
    function scroll() {
      scrolling = true
      if (frame !== undefined) cancelAnimationFrame(frame)
      frame = undefined
      clearTimeout(settled)
      settled = setTimeout(() => {
        scrolling = false
        schedule()
      }, 100)
    }
    scrollTarget.addEventListener('scroll', scroll, { passive: true })
    // Percentage root margins use the root's width, including vertical margins.
    // Use bounded pixels to keep wide chat columns from starting distant players.
    const ahead = Math.min(1200, Math.max(600, (root?.clientHeight ?? innerHeight) * 1.5))
    group = {
      subscribers,
      pending,
      dispose() {
        if (frame !== undefined) cancelAnimationFrame(frame)
        clearTimeout(settled)
        pending.clear()
        scrollTarget.removeEventListener('scroll', scroll)
      },
      preload: new IntersectionObserver(
        (entries) => {
          for (const entry of entries)
            subscribers.get(entry.target)?.({ preload: entry.isIntersecting })
        },
        {
          root,
          rootMargin: `${Math.round(ahead)}px 0px`,
        },
      ),
      visible: new IntersectionObserver(
        (entries) => {
          for (const entry of entries) {
            if (entry.isIntersecting) pending.add(entry.target)
            else {
              pending.delete(entry.target)
              subscribers.get(entry.target)?.({ visible: false })
            }
          }
          schedule()
        },
        { root, rootMargin: '120px 0px' },
      ),
    }
    groups.set(root, group)
  }
  group.subscribers.set(element, update)
  group.preload.observe(element)
  group.visible.observe(element)
  let released = false
  return () => {
    if (released) return
    released = true
    group.subscribers.delete(element)
    group.pending.delete(element)
    group.preload.unobserve(element)
    group.visible.unobserve(element)
    if (!group.subscribers.size) {
      group.preload.disconnect()
      group.visible.disconnect()
      group.dispose()
      groups.delete(root)
    }
  }
}

export function useEmbedVisibility(target: Ref<HTMLElement | null>) {
  const preload = shallowRef(false)
  const visible = shallowRef(false)
  let stop: (() => void) | undefined
  onMounted(() => {
    if (!('IntersectionObserver' in window)) {
      preload.value = visible.value = true
      return
    }
    stop = watch(
      target,
      (element, _previous, cleanup) => {
        preload.value = visible.value = false
        if (element)
          cleanup(
            observe(element, (value) => {
              if (value.preload !== undefined) preload.value = value.preload
              if (value.visible !== undefined) visible.value = value.visible
            }),
          )
      },
      { immediate: true, flush: 'post' },
    )
  })
  onBeforeUnmount(() => stop?.())
  return { preload, visible }
}
