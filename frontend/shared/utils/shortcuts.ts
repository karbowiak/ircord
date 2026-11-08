/**
 * Keyboard shortcuts: one definition drives both the handlers and the help list, so what the
 * help shows is what the keys do.
 *
 * `mod` is Ctrl or ⌘ (either is accepted on every platform; the label follows the platform).
 * Modifiers must match exactly — Ctrl Shift K is not Ctrl K, and Ctrl Alt (AltGr on Windows)
 * never matches a Ctrl shortcut, so typing characters with AltGr keeps working. Shift is not
 * checked for punctuation, because many layouts need it to type “?” or “/” at all.
 */
export interface Chord {
  key: string
  mod?: boolean
  shift?: boolean
  alt?: boolean
}
export type KeyLike = Pick<
  KeyboardEvent,
  'key' | 'code' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey'
>

export const shortcuts = {
  search: [{ key: 'k', mod: true }],
  gifs: [{ key: 'g', mod: true }],
  settings: [{ key: ',', mod: true }],
  help: [{ key: '?' }, { key: '/', mod: true }],
  previous: [{ key: 'ArrowUp', alt: true }],
  next: [{ key: 'ArrowDown', alt: true }],
  previousUnread: [{ key: 'ArrowUp', alt: true, shift: true }],
  nextUnread: [{ key: 'ArrowDown', alt: true, shift: true }],
  markRead: [{ key: 'Escape' }],
  markNetworkRead: [{ key: 'Escape', shift: true }],
  bold: [{ key: 'b', mod: true }],
  italic: [{ key: 'i', mod: true }],
  underline: [{ key: 'u', mod: true }],
} satisfies Record<string, Chord[]>
export type ShortcutName = keyof typeof shortcuts

const letter = /^[a-z]$/

/** The pressed key, by its US-layout letter for non-Latin layouts (Ctrl+К is Ctrl+K). */
function pressed(event: KeyLike): string {
  const key = event.key.length === 1 ? event.key.toLowerCase() : event.key
  if (/^\p{L}$/u.test(key) && !letter.test(key) && /^Key[A-Z]$/.test(event.code))
    return event.code.slice(3).toLowerCase()
  return key
}

export function matchesChord(event: KeyLike, chord: Chord): boolean {
  const mods = Number(event.ctrlKey) + Number(event.metaKey)
  if (chord.mod ? mods !== 1 : mods !== 0) return false
  if (event.altKey !== !!chord.alt) return false
  const key = pressed(event)
  const punctuation = chord.key.length === 1 && !letter.test(chord.key)
  if (!punctuation && event.shiftKey !== !!chord.shift) return false
  return key === chord.key
}

/** Which of the given shortcuts this key press is, if any. */
export function shortcutOf<N extends ShortcutName>(event: KeyLike, names: readonly N[]) {
  return names.find((name) => shortcuts[name].some((chord) => matchesChord(event, chord)))
}

export const isMac = (platform: string) => /Mac|iPhone|iPad/.test(platform)

const keyNames: Record<string, string> = {
  ArrowUp: '↑',
  ArrowDown: '↓',
  ArrowLeft: '←',
  ArrowRight: '→',
  Escape: 'Esc',
  Enter: 'Enter',
  Tab: 'Tab',
  Shift: 'Shift',
}

/** “Ctrl K” on Windows and Linux, “⌘ K” on Apple platforms. */
export function chordLabel(chord: Chord, mac: boolean): string {
  return [
    chord.mod && (mac ? '⌘' : 'Ctrl'),
    chord.alt && (mac ? '⌥' : 'Alt'),
    chord.shift && (mac ? '⇧' : 'Shift'),
    chord.key === 'Shift' && mac ? '⇧' : (keyNames[chord.key] ?? chord.key.toUpperCase()),
  ]
    .filter(Boolean)
    .join(' ')
}
export const shortcutLabel = (name: ShortcutName, mac: boolean) =>
  shortcuts[name].map((chord) => chordLabel(chord, mac)).join(' · ')

/** The aria-keyshortcuts value, e.g. “Control+G” or “Meta+G”. */
export function ariaShortcut(name: ShortcutName, mac: boolean): string {
  return shortcuts[name]
    .map((chord: Chord) =>
      [
        chord.mod && (mac ? 'Meta' : 'Control'),
        chord.alt && 'Alt',
        chord.shift && 'Shift',
        chord.key.length === 1 ? chord.key.toUpperCase() : chord.key,
      ]
        .filter(Boolean)
        .join('+'),
    )
    .join(' ')
}
