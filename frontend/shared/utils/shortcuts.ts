/**
 * Keyboard shortcuts: one definition drives both the handlers and the help list, so what the
 * help shows is what the keys do.
 *
 * Every action has default chords; your own bindings (saved with your account preferences) only
 * record the actions you changed, so new defaults still reach you. `resolveShortcuts` merges the
 * two, and handlers, labels and the help list all read the merged table.
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

/** The default chords of every action. An empty list means the action starts without one. */
export const defaultShortcuts = {
  search: [{ key: 'k', mod: true }],
  previous: [{ key: 'ArrowUp', alt: true }],
  next: [{ key: 'ArrowDown', alt: true }],
  previousUnread: [{ key: 'ArrowUp', alt: true, shift: true }],
  nextUnread: [{ key: 'ArrowDown', alt: true, shift: true }],
  previousNetwork: [{ key: 'ArrowUp', mod: true, alt: true }],
  nextNetwork: [{ key: 'ArrowDown', mod: true, alt: true }],
  jumpToUnread: [{ key: 'PageUp', shift: true }],
  jumpToPresent: [{ key: 'PageDown', shift: true }],
  markRead: [{ key: 'Escape' }],
  markNetworkRead: [{ key: 'Escape', shift: true }],
  focusComposer: [{ key: 'c', alt: true, shift: true }],
  gifs: [{ key: 'g', mod: true }],
  bold: [{ key: 'b', mod: true }],
  italic: [{ key: 'i', mod: true }],
  underline: [{ key: 'u', mod: true }],
  toggleMembers: [],
  inbox: [],
  saved: [],
  join: [],
  settings: [{ key: ',', mod: true }],
  help: [{ key: '?' }, { key: '/', mod: true }],
} satisfies Record<string, Chord[]>
export type ShortcutName = keyof typeof defaultShortcuts
export type ShortcutBindings = Record<ShortcutName, Chord[]>
/** Only the actions you changed; an empty list clears an action. */
export type ShortcutOverrides = Partial<Record<ShortcutName, Chord[]>>

/** Fixed numbered channel navigation, separate from the configurable actions. */
export const channelShortcuts: Chord[] = Array.from({ length: 9 }, (_, index) => ({
  key: String(index + 1),
  mod: true,
}))

export type ShortcutGroup = 'Getting around' | 'Reading' | 'Writing' | 'App'
/** What each action is called in Settings and the help list, in display order. */
export const shortcutActions: { name: ShortcutName; label: string; group: ShortcutGroup }[] = [
  { name: 'search', label: 'Search and quick switcher', group: 'Getting around' },
  { name: 'previous', label: 'Previous conversation', group: 'Getting around' },
  { name: 'next', label: 'Next conversation', group: 'Getting around' },
  { name: 'previousUnread', label: 'Previous unread or mention', group: 'Getting around' },
  { name: 'nextUnread', label: 'Next unread or mention', group: 'Getting around' },
  { name: 'previousNetwork', label: 'Previous network', group: 'Getting around' },
  { name: 'nextNetwork', label: 'Next network', group: 'Getting around' },
  { name: 'inbox', label: 'Inbox', group: 'Getting around' },
  { name: 'saved', label: 'Saved messages', group: 'Getting around' },
  { name: 'join', label: 'Join a channel', group: 'Getting around' },
  { name: 'jumpToUnread', label: 'Jump to the first unread message', group: 'Reading' },
  { name: 'jumpToPresent', label: 'Jump to the newest message', group: 'Reading' },
  { name: 'markRead', label: 'Mark conversation as read', group: 'Reading' },
  { name: 'markNetworkRead', label: 'Mark network as read', group: 'Reading' },
  { name: 'toggleMembers', label: 'Show or hide the member list', group: 'Reading' },
  { name: 'focusComposer', label: 'Focus the message box', group: 'Writing' },
  { name: 'gifs', label: 'GIF picker', group: 'Writing' },
  { name: 'bold', label: 'Bold', group: 'Writing' },
  { name: 'italic', label: 'Italic', group: 'Writing' },
  { name: 'underline', label: 'Underline', group: 'Writing' },
  { name: 'settings', label: 'Settings', group: 'App' },
  { name: 'help', label: 'Keyboard shortcuts', group: 'App' },
]
export const shortcutGroups: ShortcutGroup[] = ['Getting around', 'Reading', 'Writing', 'App']

const letter = /^[a-z]$/

/** The pressed key, by its US-layout letter for non-Latin layouts (Ctrl+К is Ctrl+K). */
function pressed(event: KeyLike): string {
  const key = event.key.length === 1 ? event.key.toLowerCase() : event.key
  if (/^\p{L}$/u.test(key) && !letter.test(key) && /^Key[A-Z]$/.test(event.code))
    return event.code.slice(3).toLowerCase()
  return key
}
/** One character that is not a letter: its Shift state depends on the keyboard layout. */
const isPunctuation = (key: string) => [...key].length === 1 && !letter.test(key)

export function matchesChord(event: KeyLike, chord: Chord): boolean {
  const mods = Number(event.ctrlKey) + Number(event.metaKey)
  if (chord.mod ? mods !== 1 : mods !== 0) return false
  if (event.altKey !== !!chord.alt) return false
  const key = pressed(event)
  if (!isPunctuation(chord.key) && event.shiftKey !== !!chord.shift) return false
  return key === chord.key
}

/** Which of the given shortcuts this key press is, if any. */
export function shortcutOf<N extends ShortcutName>(
  event: KeyLike,
  names: readonly N[],
  bindings: ShortcutBindings = defaultShortcuts,
) {
  return names.find((name) => bindings[name].some((chord) => matchesChord(event, chord)))
}

// ---- Your own bindings ------------------------------------------------------------------------

const modifierKeys = new Set([
  'Shift',
  'Control',
  'Alt',
  'Meta',
  'AltGraph',
  'OS',
  'Hyper',
  'Super',
])
const maxChords = 3

/** The chord a key press makes, for recording a binding; null while only modifiers are held. */
export function chordOf(event: KeyLike): Chord | null {
  if (modifierKeys.has(event.key) || event.key === 'Dead' || event.key === 'Process') return null
  if (event.key === 'Unidentified' || !event.key) return null
  // With Ctrl and ⌘ both held there is nothing matchesChord would ever accept.
  if (event.ctrlKey && event.metaKey) return null
  const key = pressed(event)
  const chord: Chord = { key }
  if (event.ctrlKey || event.metaKey) chord.mod = true
  if (event.altKey) chord.alt = true
  // A layout's Shift for “?” is part of the character, not of the shortcut.
  if (event.shiftKey && !isPunctuation(key)) chord.shift = true
  return chord
}

export const sameChord = (a: Chord, b: Chord) =>
  a.key === b.key && !!a.mod === !!b.mod && !!a.shift === !!b.shift && !!a.alt === !!b.alt

/**
 * Why this chord can't be a shortcut, or null when it can. Keys that type text, move the caret,
 * send, complete or edit messages, copy and paste, or move focus keep their usual meaning.
 */
export function reservedReason(chord: Chord): string | null {
  const plain = !chord.mod && !chord.alt
  if (plain && [...chord.key].length === 1) return 'This key types text. Add Ctrl, Alt or ⌘ to it.'
  if (chord.key === 'Tab') return 'Tab moves between controls and completes names.'
  if (chord.key === 'Enter') return 'Enter sends messages and adds new lines.'
  if (chord.key === 'F10' && chord.shift && plain) return 'Shift F10 opens context menus.'
  const caret = [
    'ArrowUp',
    'ArrowDown',
    'ArrowLeft',
    'ArrowRight',
    'Home',
    'End',
    'Backspace',
    'Delete',
  ]
  if (caret.includes(chord.key) && (plain || (chord.mod && !chord.alt)))
    return 'This key moves the caret or edits text.'
  if (['PageUp', 'PageDown'].includes(chord.key) && plain && !chord.shift)
    return 'This key scrolls the conversation.'
  if (chord.mod && !chord.alt && !chord.shift && ['a', 'c', 'v', 'x', 'z', 'y'].includes(chord.key))
    return 'This key selects, copies, pastes or undoes.'
  if (chord.mod && !chord.alt && chord.shift && ['z', 'v'].includes(chord.key))
    return 'This key redoes or pastes plain text.'
  return null
}

/** The actions, other than `except`, that already use this chord. */
export function chordUsers(bindings: ShortcutBindings, chord: Chord, except?: ShortcutName) {
  return shortcutActions
    .map((action) => action.name)
    .filter((name) => name !== except && bindings[name].some((c) => sameChord(c, chord)))
}

export function resolveShortcuts(overrides: ShortcutOverrides | undefined): ShortcutBindings {
  const result = { ...defaultShortcuts } as ShortcutBindings
  for (const action of shortcutActions) {
    const own = overrides?.[action.name]
    if (own) result[action.name] = own
  }
  // A changed binding wins over a default it collides with, so one key never runs two actions.
  for (const action of shortcutActions) {
    if (overrides?.[action.name]) continue
    const taken = (chord: Chord) =>
      shortcutActions.some(
        (other) =>
          other.name !== action.name && overrides?.[other.name]?.some((c) => sameChord(c, chord)),
      )
    if (result[action.name].some(taken))
      result[action.name] = result[action.name].filter((chord) => !taken(chord))
  }
  return result
}

/** The overrides after setting one action's chords; matching the defaults drops the override. */
export function withBinding(
  overrides: ShortcutOverrides,
  name: ShortcutName,
  chords: Chord[],
): ShortcutOverrides {
  const next = { ...overrides }
  const defaults: Chord[] = defaultShortcuts[name]
  if (chords.length === defaults.length && chords.every((c, i) => sameChord(c, defaults[i]!)))
    delete next[name]
  else next[name] = chords
  return next
}

/** Keeps only well-formed bindings of known actions, each chord used once. */
export function sanitizeShortcuts(input: unknown): ShortcutOverrides {
  if (!input || typeof input !== 'object') return {}
  const v = input as Record<string, unknown>
  const result: ShortcutOverrides = {}
  const used: Chord[] = []
  for (const { name } of shortcutActions) {
    const list = v[name]
    if (!Array.isArray(list)) continue
    const chords: Chord[] = []
    for (const raw of list.slice(0, maxChords)) {
      if (!raw || typeof raw !== 'object') continue
      const c = raw as Record<string, unknown>
      if (typeof c.key !== 'string') continue
      // One character (“k”, “,”) or a named key (“ArrowUp”, “F5”, “PageDown”).
      // eslint-disable-next-line no-control-regex
      const single = [...c.key].length === 1 && !/[\x00-\x1f\x7f]/.test(c.key)
      if (!single && !/^[A-Za-z][A-Za-z0-9]{1,19}$/.test(c.key)) continue
      const chord: Chord = { key: single ? c.key.toLowerCase() : c.key }
      if (c.mod === true) chord.mod = true
      if (c.shift === true && !isPunctuation(chord.key)) chord.shift = true
      if (c.alt === true) chord.alt = true
      if (reservedReason(chord) && !defaultShortcuts[name].some((d: Chord) => sameChord(d, chord)))
        continue
      if (used.some((u) => sameChord(u, chord))) continue
      used.push(chord)
      chords.push(chord)
    }
    // Only an empty list on purpose clears an action; bad chords alone keep the defaults.
    if (list.length && !chords.length) continue
    result[name] = chords
  }
  return result
}

// ---- Labels -----------------------------------------------------------------------------------

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
  PageUp: 'Page Up',
  PageDown: 'Page Down',
  ' ': 'Space',
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
/** “Ctrl K”, several chords joined by “ · ”, or '' for an action without one. */
export const shortcutLabel = (
  name: ShortcutName,
  mac: boolean,
  bindings: ShortcutBindings = defaultShortcuts,
) => bindings[name].map((chord) => chordLabel(chord, mac)).join(' · ')

/** The aria-keyshortcuts value, e.g. “Control+G” or “Meta+G”; undefined without a chord. */
export function ariaShortcut(
  name: ShortcutName,
  mac: boolean,
  bindings: ShortcutBindings = defaultShortcuts,
): string | undefined {
  const value = bindings[name]
    .map((chord: Chord) =>
      [
        chord.mod && (mac ? 'Meta' : 'Control'),
        chord.alt && 'Alt',
        chord.shift && 'Shift',
        chord.key === ' ' ? 'Space' : chord.key.length === 1 ? chord.key.toUpperCase() : chord.key,
      ]
        .filter(Boolean)
        .join('+'),
    )
    .join(' ')
  return value || undefined
}
