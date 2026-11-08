/** The parts of a keydown that decide whether it types text. */
export interface TypedKey {
  key: string
  ctrlKey: boolean
  metaKey: boolean
  altKey: boolean
  isComposing: boolean
  getModifierState?: (key: string) => boolean
}

/**
 * Whether a keydown, landing outside any text field, is the start of typed text that belongs in
 * the composer. Shortcuts (Ctrl/⌘ combinations, Alt on Windows and Linux) and named keys (Enter,
 * arrows, F-keys) are not text. AltGr (reported as Ctrl+Alt on Windows) and macOS Option produce
 * characters such as @, € or å on many layouts, so they count. Dead keys and IME “Process” keys
 * start a composition, which must begin in the composer for the accent or character to arrive.
 * `?` stays the shortcut-help key.
 */
export function startsTyping(event: TypedKey, apple = false): boolean {
  if (event.isComposing) return false
  const altGraph = event.getModifierState?.('AltGraph') ?? false
  if (!altGraph) {
    if (event.ctrlKey || event.metaKey) return false
    if (event.altKey && !apple) return false
  }
  if (event.key === 'Dead' || event.key === 'Process') return true
  if (event.key === '?') return false
  // Named key values (Enter, ArrowUp, F5, Unidentified) are ASCII words. Text is one character,
  // or anything non-ASCII: emoji sequences, Indic conjuncts and decomposed accents can span
  // several code points.
  // eslint-disable-next-line no-control-regex
  return [...event.key].length === 1 || /[^\x00-\x7f]/.test(event.key)
}
