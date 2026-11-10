import {
  ariaShortcut,
  isMac,
  resolveShortcuts,
  shortcutLabel,
  withBinding,
  type Chord,
  type ShortcutName,
} from '../../../shared/utils/shortcuts'

/**
 * Your keyboard shortcuts: the defaults with your own changes on top, saved with your account
 * preferences. Labels follow the platform (Ctrl on Windows and Linux, ⌘ on Apple platforms).
 */
export function useShortcuts() {
  const { t } = useI18n()
  const preferences = usePreferencesStore()
  const mac = typeof navigator !== 'undefined' && isMac(navigator.platform)
  const bindings = computed(() => resolveShortcuts(preferences.preferences.shortcuts))
  const overrides = () => preferences.preferences.shortcuts ?? {}
  return {
    mac,
    bindings,
    label: (name: ShortcutName) => shortcutLabel(name, mac, bindings.value, t),
    aria: (name: ShortcutName) => ariaShortcut(name, mac, bindings.value),
    /** “Settings (Ctrl ,)”, or just the text when the action has no keys. */
    hinted: (text: string, name: ShortcutName) => {
      const keys = shortcutLabel(name, mac, bindings.value, t)
      return keys ? t('workspaceUi.shortcuts.hinted', { label: text, keys }) : text
    },
    /** Sets one action's chords; matching its defaults stores nothing for it. */
    set: (name: ShortcutName, chords: Chord[]) =>
      preferences.update({ shortcuts: withBinding(overrides(), name, chords) }),
    /** Sets several actions at once, e.g. moving a chord from one action to another. */
    setMany: (changes: Partial<Record<ShortcutName, Chord[]>>) => {
      let next = overrides()
      for (const [name, chords] of Object.entries(changes))
        next = withBinding(next, name as ShortcutName, chords!)
      preferences.update({ shortcuts: next })
    },
    reset: (name: ShortcutName) => {
      const next = { ...overrides() }
      delete next[name]
      preferences.update({ shortcuts: next })
    },
    resetAll: () => preferences.update({ shortcuts: {} }),
    changed: (name: ShortcutName) => !!overrides()[name],
    anyChanged: computed(() => Object.keys(preferences.preferences.shortcuts ?? {}).length > 0),
  }
}
