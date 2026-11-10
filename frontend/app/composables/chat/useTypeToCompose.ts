import { isEditable, overlayOpen } from '../../../shared/utils/focus'
import { startsTyping } from '../../../shared/utils/type-to-compose'

/** Focused widgets that use printable keys themselves (type-ahead, roving focus). */
const KEYED_WIDGETS =
  '[role="listbox"], [role="menu"], [role="menubar"], [role="grid"], [role="tree"], [role="treegrid"], [role="radiogroup"], [role="tablist"], [role="slider"]'

const apple = () =>
  typeof navigator !== 'undefined' && /Mac|iPhone|iPad|iPod/.test(navigator.platform)

/**
 * Route plain typing to the composer while preserving dialogs, selection and shortcuts.
 *
 * The composer is focused during keydown, before the browser inserts the character, so the first
 * key, the draft and its remembered caret are kept without synthesizing text or input events.
 * The listener only ever sees keys sent to this document while it has focus.
 */
export function useTypeToCompose(input: Ref<HTMLTextAreaElement | null>) {
  function keydown(event: KeyboardEvent) {
    const field = input.value
    if (!field || field.disabled || event.defaultPrevented) return
    if (!startsTyping(event, apple())) return
    const target = event.target instanceof Element ? event.target : null
    if (target === field || isEditable(target)) return
    if (target?.closest(KEYED_WIDGETS)) return
    // Space presses a focused control and pages a focused scroll region (the timeline); it only
    // starts a message when nothing in particular has focus.
    if (event.key === ' ' && target && target !== document.body) return
    if (overlayOpen()) return
    // An intentional selection (e.g. message text about to be copied) stays as it is.
    if (!(window.getSelection()?.isCollapsed ?? true)) return
    // Defence in depth: focus() would be a no-op anyway while an open mobile drawer makes the
    // main column inert, or while the composer is hidden or detached.
    if (!field.isConnected || field.closest('[inert]')) return
    if (field.checkVisibility && !field.checkVisibility({ visibilityProperty: true })) return
    field.focus({ preventScroll: true })
  }
  useEventListener('keydown', keydown, { capture: true })
}
