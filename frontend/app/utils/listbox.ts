// Keyboard behaviour of a select-only combobox (the WAI-ARIA APG pattern), kept free of the DOM
// so AppSelect and its tests share one implementation.

export interface ListboxOption {
  label: string
  disabled?: boolean
}

export type ListboxAction =
  | { type: 'open'; active: number }
  | { type: 'move'; active: number }
  | { type: 'select'; index: number; close: true; keepFocus: boolean }
  | { type: 'close' }
  | { type: 'none' }

const pageSize = 10

/** The nearest enabled option from `from` in `step` direction, or `from` itself if none. */
export function enabledFrom(options: readonly ListboxOption[], from: number, step: 1 | -1) {
  for (let i = from; i >= 0 && i < options.length; i += step) if (!options[i]!.disabled) return i
  return -1
}

function clampEnabled(options: readonly ListboxOption[], index: number, step: 1 | -1) {
  const bounded = Math.max(0, Math.min(options.length - 1, index))
  const found = enabledFrom(options, bounded, step)
  return found >= 0 ? found : enabledFrom(options, bounded, step === 1 ? -1 : 1)
}

/**
 * Type-ahead: the next enabled option whose label starts with `query`, searching after `from`
 * and wrapping. Repeating one letter ("ccc") cycles through the options starting with it.
 */
export function typeahead(options: readonly ListboxOption[], query: string, from: number) {
  if (!query || !options.length) return -1
  const lower = query.toLowerCase()
  const repeated = [...lower].every((c) => c === lower[0])
  const needle = repeated ? lower[0]! : lower
  // A longer query may still match the current option; a single letter moves past it.
  const start = repeated ? from + 1 : Math.max(from, 0)
  for (let n = 0; n < options.length; n++) {
    const i = (start + n + options.length) % options.length
    const option = options[i]!
    if (!option.disabled && option.label.toLowerCase().startsWith(needle)) return i
  }
  return -1
}

/**
 * What a key press does. `active` is the highlighted option while open; `selected` is the
 * current value's index (-1 for none).
 */
export function listboxKey(
  key: string,
  state: { open: boolean; active: number; selected: number; altKey?: boolean },
  options: readonly ListboxOption[],
): ListboxAction {
  const last = options.length - 1
  const { open, active, selected } = state
  if (!options.length) return { type: 'none' }
  const current = selected >= 0 ? selected : enabledFrom(options, 0, 1)
  if (!open) {
    switch (key) {
      case 'ArrowDown':
      case 'ArrowUp':
      case 'Enter':
      case ' ':
        return { type: 'open', active: current }
      case 'Home':
        return { type: 'open', active: enabledFrom(options, 0, 1) }
      case 'End':
        return { type: 'open', active: enabledFrom(options, last, -1) }
      default:
        return { type: 'none' }
    }
  }
  switch (key) {
    case 'ArrowDown':
      return { type: 'move', active: clampEnabled(options, active + 1, 1) }
    case 'ArrowUp':
      if (state.altKey && active >= 0)
        return { type: 'select', index: active, close: true, keepFocus: true }
      return { type: 'move', active: clampEnabled(options, active - 1, -1) }
    case 'PageDown':
      return { type: 'move', active: clampEnabled(options, active + pageSize, 1) }
    case 'PageUp':
      return { type: 'move', active: clampEnabled(options, active - pageSize, -1) }
    case 'Home':
      return { type: 'move', active: enabledFrom(options, 0, 1) }
    case 'End':
      return { type: 'move', active: enabledFrom(options, last, -1) }
    case 'Enter':
    case ' ':
      return active >= 0 && !options[active]?.disabled
        ? { type: 'select', index: active, close: true, keepFocus: true }
        : { type: 'close' }
    // Tab keeps the highlighted choice and lets focus move on.
    case 'Tab':
      return active >= 0 && active !== selected && !options[active]?.disabled
        ? { type: 'select', index: active, close: true, keepFocus: false }
        : { type: 'close' }
    case 'Escape':
      return { type: 'close' }
    default:
      return { type: 'none' }
  }
}

/** Whether a key press is a printable character that should drive type-ahead. */
export const isTypeaheadKey = (event: {
  key: string
  ctrlKey: boolean
  metaKey: boolean
  altKey: boolean
}) => event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey

/** Where a popup goes: below its trigger, or above when there's more room there. */
export function placePopup(
  trigger: { top: number; bottom: number; left: number; width: number },
  viewport: { width: number; height: number },
  wanted: number,
  gap = 4,
  margin = 8,
) {
  const below = viewport.height - trigger.bottom - gap - margin
  const above = trigger.top - gap - margin
  const up = below < Math.min(wanted, 160) && above > below
  const maxHeight = Math.max(0, Math.min(wanted, up ? above : below))
  const width = Math.min(Math.max(trigger.width, 160), viewport.width - margin * 2)
  const left = Math.max(margin, Math.min(trigger.left, viewport.width - margin - width))
  return up
    ? { up, left, width, maxHeight, bottom: viewport.height - trigger.top + gap }
    : { up, left, width, maxHeight, top: trigger.bottom + gap }
}
