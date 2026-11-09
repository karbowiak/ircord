/**
 * Where keyboard input belongs. Type-to-compose, the workspace shortcuts and the update reload all
 * ask the same two questions — is focus in something that edits text, and is an overlay holding
 * the keyboard — so they share one answer.
 */

/** Anything that edits text itself. */
export const EDITABLE =
  'input, textarea, select, [contenteditable]:not([contenteditable="false"]), [role="textbox"], [role="searchbox"], [role="combobox"], [role="spinbutton"]'

/** Whether keys sent to this target are edited text there (fields, custom editors, designMode). */
export function isEditable(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false
  if (target instanceof HTMLElement && target.isContentEditable) return true
  return !!target.closest(EDITABLE)
}

/** Open dialogs, menus, listboxes and pickers keep the keyboard, whatever is focused inside them.
 * A field's inline suggestions (`data-inline-suggestions`) don't: they handle their own keys. */
const OVERLAYS =
  'dialog[open], [aria-modal="true"], [role="dialog"], [role="alertdialog"], [role="menu"], [role="listbox"]:not([data-inline-suggestions])'

export function overlayOpen(): boolean {
  if (document.querySelector(OVERLAYS)) return true
  // Browsers without the Popover API reject the selector; they cannot have one open either.
  try {
    return !!document.querySelector(':popover-open')
  } catch {
    return false
  }
}
