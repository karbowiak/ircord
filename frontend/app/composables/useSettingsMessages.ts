import type { SettingsMessages } from '../../shared/settings-index'

/**
 * The settings index's catalog access in the active locale. Call it in setup: the returned
 * functions read the locale when they run, so search results follow a locale change.
 */
export function useSettingsMessages(): SettingsMessages {
  const { t, te, tm, rt } = useI18n()
  return {
    t: (key) => t(key),
    te: (key) => te(key),
    list: (key) => {
      const messages = tm(key)
      return Array.isArray(messages) ? messages.map((message) => rt(message)) : []
    },
  }
}
