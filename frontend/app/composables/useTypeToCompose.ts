import { startsTyping } from '../../shared/utils/type-to-compose'

/** Anything that edits text itself; typing there is never redirected. */
const EDITABLE =
  'input, textarea, select, [contenteditable]:not([contenteditable="false"]), [role="textbox"], [role="searchbox"], [role="combobox"], [role="spinbutton"]'
/** Focused widgets that use printable keys themselves (type-ahead, roving focus). */
const KEYED_WIDGETS =
  '[role="listbox"], [role="menu"], [role="menubar"], [role="grid"], [role="tree"], [role="treegrid"], [role="radiogroup"], [role="tablist"], [role="slider"]'
/** Controls that Space activates. */
const SPACE_ACTIVATED =
  'button, a, summary, [role="button"], [role="link"], [role="checkbox"], [role="switch"], [role="radio"], [role="tab"], [role="option"], [role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"]'
/** Open overlays keep the keyboard, even while focus is on one of their buttons. */
const OVERLAYS =
  'dialog[open], [aria-modal="true"], [role="dialog"], [role="alertdialog"], [role="menu"], [role="listbox"]'

function overlayOpen() {
  if (document.querySelector(OVERLAYS)) return true
  // Browsers without the Popover API reject the selector; they cannot have one open either.
  try {
    return !!document.querySelector(':popover-open')
  } catch {
    return false
  }
}

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
    if (target === field || target?.closest(EDITABLE)) return
    if (target instanceof HTMLElement && target.isContentEditable) return
    if (target?.closest(KEYED_WIDGETS)) return
    if (event.key === ' ' && target?.closest(SPACE_ACTIVATED)) return
    if (overlayOpen()) return
    // An intentional selection (e.g. message text about to be copied) stays as it is.
    if (!(window.getSelection()?.isCollapsed ?? true)) return
    // A hidden or inert composer (closed mobile drawers cover it) cannot take the text.
    if (!field.isConnected || field.closest('[inert]')) return
    if (field.checkVisibility && !field.checkVisibility({ visibilityProperty: true })) return
    field.focus({ preventScroll: true })
  }
  onMounted(() => window.addEventListener('keydown', keydown, true))
  onBeforeUnmount(() => window.removeEventListener('keydown', keydown, true))
}
