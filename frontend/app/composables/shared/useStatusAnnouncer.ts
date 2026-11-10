type Politeness = 'polite' | 'assertive'

/**
 * Status changes for screen readers, through the one persistent Nuxt announcer in app.vue. Live
 * regions that mount together with their text are often not read, so toasts, banners and result
 * counts announce here instead of carrying their own role="status". `later` waits for the text
 * to settle (typing in a filter is announced once); its timer clears when the caller unmounts.
 */
export function useStatusAnnouncer(delay = 800) {
  const announcer = useAnnouncer()
  let timer: ReturnType<typeof setTimeout> | undefined
  function say(text: string, politeness: Politeness = 'polite') {
    clearTimeout(timer)
    if (!text) return
    // Clear first, so the same text twice is still a change the screen reader hears.
    announcer.set('', politeness)
    void nextTick(() => announcer.set(text, politeness))
  }
  function later(text: string, politeness: Politeness = 'polite') {
    clearTimeout(timer)
    timer = setTimeout(() => say(text, politeness), delay)
  }
  const cancel = () => clearTimeout(timer)
  if (getCurrentScope()) onScopeDispose(cancel)
  return { say, later, cancel }
}

/**
 * Announces `text` when it changes after the first render, once it settles. Text present when the
 * caller mounts is not news, and neither is a change caused by `context` (another conversation,
 * network or tab): those cancel instead. Empty text announces nothing.
 */
export function useAnnouncedStatus(
  text: () => string,
  options: { context?: () => unknown; delay?: number; politeness?: () => Politeness } = {},
) {
  const { later, cancel } = useStatusAnnouncer(options.delay)
  watch([text, () => options.context?.()], ([next, context], [, previousContext]) => {
    if (context !== previousContext || !next) return cancel()
    later(next, options.politeness?.())
  })
}
