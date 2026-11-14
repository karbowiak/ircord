/**
 * Settings search: one entry per control in Settings, with the words people use for it.
 *
 * Each entry's `id` is also the `data-setting` attribute on that control's row, so a search
 * result (or a direct link) can open the section and land on the row. Each entry's text lives in
 * the English catalog under `settings.index.<id>`, so search follows the active locale: `label`
 * is required, while `title`, `description`, `group`, `keywords` and `unavailable` appear only
 * when the catalog has them. Labels are written to make sense without the panel around them
 * (“Notification sound” for the row titled “Sound”); descriptions are short search summaries, not
 * copies of the hint text. Only static wording lives in the catalog: never account values, field
 * contents or anything else a user typed.
 */

/** The headings the sections are listed under in the settings nav: catalog keys. */
export const settingsGroups = [
  { id: 'user', name: 'settings.groups.user' },
  { id: 'app', name: 'settings.groups.app' },
  { id: 'server', name: 'settings.groups.server' },
] as const

/** In nav order. Each section has one task; every setting has one home in one of them. */
export const settingsSections = [
  { id: 'profile', name: 'settings.sections.profile', icon: 'user', group: 'user' },
  { id: 'account', name: 'settings.sections.account', icon: 'key', group: 'user' },
  { id: 'devices', name: 'settings.sections.devices', icon: 'monitor', group: 'user' },
  { id: 'networks', name: 'settings.sections.networks', icon: 'globe', group: 'user' },
  { id: 'appearance', name: 'settings.sections.appearance', icon: 'palette', group: 'app' },
  { id: 'messages', name: 'settings.sections.messages', icon: 'message', group: 'app' },
  { id: 'notifications', name: 'settings.sections.notifications', icon: 'bell', group: 'app' },
  { id: 'read-aloud', name: 'settings.sections.readAloud', icon: 'read-aloud', group: 'app' },
  { id: 'shortcuts', name: 'settings.sections.shortcuts', icon: 'keyboard', group: 'app' },
  { id: 'emotes', name: 'settings.sections.emotes', icon: 'smile', group: 'server' },
  { id: 'about', name: 'settings.sections.about', icon: 'info', group: 'server' },
] as const

export type SettingsSectionId = (typeof settingsSections)[number]['id']

/** Where a group's values are kept, shown beside its heading: catalog keys. */
export const scopeLabels = {
  device: 'settings.scope.device',
  account: 'settings.scope.account',
} as const

/** What the current user and device can reach right now. */
export interface SettingsAccess {
  /** This account can add server emoji. */
  emoteAdd: boolean
  /** This device uses push notifications. */
  push: boolean
  /** Quiet hours are switched on. */
  quietHours: boolean
  /** Settings offers installing IRCord as an app on this device. */
  install: boolean
}

type Requirement = keyof SettingsAccess

export interface SettingEntry {
  id: string
  section: SettingsSectionId
  /**
   * Shown only when this holds. With `unavailable` in the catalog, it is listed anyway with that
   * explanation and opens `fallback`, the control that makes it available; without, it is left
   * out entirely.
   */
  requires?: Requirement
  fallback?: string
}

/** Catalog access for the index: the active locale's text, read at search time. */
export interface SettingsMessages {
  t: (key: string) => string
  te: (key: string) => boolean
  list: (key: string) => string[]
}

/** An entry's text in the active locale. */
export interface SettingText {
  label: string
  title?: string
  description?: string
  group?: string
  keywords: string[]
  unavailable?: string
}

export const settingsIndex: readonly SettingEntry[] = [
  // Appearance
  { id: 'interface-scale', section: 'appearance' },
  { id: 'appearance-sync', section: 'appearance' },
  { id: 'theme-mode', section: 'appearance' },
  { id: 'colour-scheme', section: 'appearance' },
  { id: 'scheme-variants', section: 'appearance' },
  { id: 'accent', section: 'appearance' },
  { id: 'interface-font', section: 'appearance' },
  { id: 'sidebar-density', section: 'appearance' },
  { id: 'corners', section: 'appearance' },
  { id: 'reduce-motion', section: 'appearance' },
  { id: 'appearance-reset', section: 'appearance' },

  // Profile
  { id: 'profile-image', section: 'profile' },
  { id: 'default-avatars', section: 'profile' },
  { id: 'display-name', section: 'profile' },
  { id: 'name-colour', section: 'profile' },
  { id: 'about-you', section: 'profile' },

  // Networks
  { id: 'network-list', section: 'networks' },
  { id: 'add-network', section: 'networks' },
  { id: 'register-channels', section: 'networks' },
  { id: 'irc-links', section: 'networks' },

  // Chat
  { id: 'message-layout', section: 'messages' },
  { id: 'message-font', section: 'messages' },
  { id: 'text-size', section: 'messages' },
  { id: 'group-spacing', section: 'messages' },
  { id: 'clock', section: 'messages' },
  { id: 'coloured-names', section: 'messages' },
  { id: 'startup-conversation', section: 'messages' },
  { id: 'channel-activity', section: 'messages' },
  { id: 'ask-before-embeds', section: 'messages' },
  { id: 'animate-media', section: 'messages' },
  { id: 'ignored-people', section: 'messages' },
  { id: 'messages-reset', section: 'messages' },

  // Notifications
  { id: 'notifications-status', section: 'notifications' },
  { id: 'install-app', section: 'notifications', requires: 'install' },
  { id: 'notifications-device', section: 'notifications' },
  {
    id: 'notifications-test',
    section: 'notifications',
    requires: 'push',
    fallback: 'notifications-device',
  },
  { id: 'notification-sound', section: 'notifications' },
  { id: 'notify-last-device', section: 'notifications' },
  { id: 'notify-all-devices', section: 'notifications' },
  { id: 'notify-reset', section: 'notifications' },
  { id: 'auto-away', section: 'notifications' },
  { id: 'quiet-hours', section: 'notifications' },
  {
    id: 'quiet-hours-times',
    section: 'notifications',
    requires: 'quietHours',
    fallback: 'quiet-hours',
  },
  { id: 'channel-notifications', section: 'notifications' },
  { id: 'direct-notifications', section: 'notifications' },
  { id: 'highlight-words', section: 'notifications' },

  // Read aloud
  { id: 'read-aloud-device', section: 'read-aloud' },
  { id: 'speech-service', section: 'read-aloud' },
  { id: 'speech-test', section: 'read-aloud' },
  { id: 'speech-remove', section: 'read-aloud' },

  // Emotes
  { id: 'add-emote', section: 'emotes', requires: 'emoteAdd' },
  { id: 'server-emotes', section: 'emotes' },

  // Account
  { id: 'password', section: 'account' },
  { id: 'log-out', section: 'account' },
  { id: 'delete-account', section: 'account' },
  { id: 'device-sessions', section: 'devices' },

  // Shortcuts and About
  { id: 'keyboard-shortcuts', section: 'shortcuts' },
  { id: 'about-ircord', section: 'about' },
]

const byId = new Map(settingsIndex.map((entry) => [entry.id, entry]))

/** The entry with this id, if there is one. */
export const settingEntry = (id: string) => byId.get(id)

/** An entry's text in the active locale. Optional fields are read only when the catalog has them. */
export function settingText(messages: SettingsMessages, entry: SettingEntry): SettingText {
  const base = `settings.index.${entry.id}`
  const optional = (field: string) =>
    messages.te(`${base}.${field}`) ? messages.t(`${base}.${field}`) : undefined
  return {
    label: messages.t(`${base}.label`),
    title: optional('title'),
    description: optional('description'),
    group: optional('group'),
    keywords: messages.list(`${base}.keywords`),
    unavailable: optional('unavailable'),
  }
}

/** Lowercase words without accents or punctuation, with a trailing plural “s” removed. */
export function settingWords(text: string): string[] {
  return text
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[’']/g, '')
    .split(/[^\p{L}\p{N}]+/u)
    .filter(Boolean)
    .map((word) =>
      word.length > 3 && word.endsWith('s') && !word.endsWith('ss') ? word.slice(0, -1) : word,
    )
}

export interface SettingResult {
  entry: SettingEntry
  /** The entry's text in the active locale. */
  text: SettingText
  /** Why it is not usable yet, when it is listed anyway. */
  unavailable?: string
  /**
   * Higher first: 3 the exact label, 2 every word in the label, 1.5 an exact synonym, 1 words
   * from the label, synonyms, group or section, 0 only with the description.
   */
  rank: number
}

/** At most this many results are listed; `total` says how many matched. */
export const MAX_RESULTS = 20

/** Every query word starts some word in `words`. */
const covers = (query: string[], words: string[]) =>
  query.every((q) => words.some((w) => w.startsWith(q)))

function rank(query: string[], phrase: string, text: SettingText, section: string): number | null {
  const titles = [text.label, text.title ?? ''].map(settingWords)
  if (titles.some((t) => t.join(' ') === phrase)) return 3
  if (titles.some((t) => t.length && covers(query, t))) return 2
  const label = titles.flat()
  const keywords = text.keywords.map(settingWords)
  if (keywords.some((k) => k.join(' ') === phrase)) return 1.5
  const named = [
    ...label,
    ...keywords.flat(),
    ...settingWords(text.group ?? ''),
    ...settingWords(section),
  ]
  if (covers(query, named)) return 1
  if (covers(query, [...named, ...settingWords(text.description ?? '')])) return 0
  return null
}

/**
 * The settings matching `query` that this user can reach, best first; ties keep the order they
 * appear in Settings. Unavailable-but-explained entries come after the usable ones of their rank.
 * At most MAX_RESULTS are returned; `total` counts every match.
 */
export function searchSettings(
  query: string,
  access: SettingsAccess,
  messages: SettingsMessages,
  index: readonly SettingEntry[] = settingsIndex,
): { results: SettingResult[]; total: number } {
  const words = settingWords(query)
  if (!words.length) return { results: [], total: 0 }
  const phrase = words.join(' ')
  const results: (SettingResult & { order: number })[] = []
  index.forEach((entry, order) => {
    const text = settingText(messages, entry)
    const missing = entry.requires && !access[entry.requires]
    if (missing && !text.unavailable) return
    const score = rank(words, phrase, text, sectionName(entry.section, messages.t))
    if (score == null) return
    results.push({
      entry,
      text,
      rank: score,
      order,
      ...(missing ? { unavailable: text.unavailable } : {}),
    })
  })
  const listed = results
    .sort(
      (a, b) =>
        b.rank - a.rank || Number(!!a.unavailable) - Number(!!b.unavailable) || a.order - b.order,
    )
    .slice(0, MAX_RESULTS)
    .map(({ order: _, ...result }) => result)
  return { results: listed, total: results.length }
}

/** A section's name in the active locale. */
export function sectionName(id: SettingsSectionId, t: (key: string) => string): string {
  const section = settingsSections.find((s) => s.id === id)
  return section ? t(section.name) : id
}

/**
 * Where a destination (a section id or a setting id) opens: its section, and the setting to
 * reveal there. An unavailable setting opens the control that enables it instead.
 */
export function settingsDestination(
  target: string,
  access: SettingsAccess,
): { section: SettingsSectionId; setting?: string } {
  const section = settingsSections.find((s) => s.id === target)
  if (section) return { section: section.id }
  const entry = byId.get(target)
  if (!entry) return { section: 'appearance' }
  if (entry.requires && !access[entry.requires]) {
    const fallback = entry.fallback ? byId.get(entry.fallback) : undefined
    return fallback
      ? { section: fallback.section, setting: fallback.id }
      : { section: entry.section }
  }
  return { section: entry.section, setting: entry.id }
}
