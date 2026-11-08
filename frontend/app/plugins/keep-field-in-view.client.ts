import { isEditable } from '../../shared/utils/focus'

/**
 * Touch screens: keep the focused field visible when the on-screen keyboard opens. iOS lays the
 * keyboard over the page without resizing it, and browsers scroll only the document, not the
 * inner scroll areas the sign-in screen and dialogs use. Only fields there are moved; the
 * composer and timeline manage their own position.
 */
export default defineNuxtPlugin(() => {
  const viewport = window.visualViewport
  if (!viewport || !window.matchMedia?.('(pointer: coarse)').matches) return
  const MARGIN = 12
  let frame = 0
  function reveal() {
    cancelAnimationFrame(frame)
    frame = requestAnimationFrame(() => {
      const field = document.activeElement
      if (!(field instanceof HTMLElement) || !isEditable(field)) return
      if (!field.closest('[role="dialog"], .login-screen')) return
      const box = field.getBoundingClientRect()
      const top = viewport!.offsetTop + MARGIN
      const bottom = viewport!.offsetTop + viewport!.height - MARGIN
      if (box.top >= top && box.bottom <= bottom) return
      field.scrollIntoView({ block: 'center', inline: 'nearest' })
    })
  }
  viewport.addEventListener('resize', reveal)
  // Once the keyboard has had time to open; the resize also covers keyboards that open later.
  document.addEventListener('focusin', () => setTimeout(reveal, 300))
})
