/**
 * Settings search: one entry per control in Settings, with the words people use for it.
 *
 * Each entry's `id` is also the `data-setting` attribute on that control's row, so a search
 * result (or a direct link) can open the section and land on the row. tests/settings-index.test.ts
 * checks the two stay in step, and that each row sits in its entry's section and shows its
 * `label` (or its on-screen `title`). Labels are written to make sense without the panel around
 * them (“Notification sound” for the row titled “Sound”); descriptions are short search summaries,
 * not copies of the hint text. Only static wording lives here: never account values, field
 * contents or anything else a user typed.
 */

/** The headings the sections are listed under in the settings nav. */
export const settingsGroups = [
  { id: 'user', name: 'User settings' },
  { id: 'app', name: 'App settings' },
  { id: 'server', name: 'IRCord' },
] as const

/** In nav order. Each section has one task; every setting has one home in one of them. */
export const settingsSections = [
  { id: 'profile', name: 'Profile', icon: 'user', group: 'user' },
  { id: 'account', name: 'Account', icon: 'key', group: 'user' },
  { id: 'networks', name: 'Networks', icon: 'globe', group: 'user' },
  { id: 'appearance', name: 'Appearance', icon: 'palette', group: 'app' },
  { id: 'messages', name: 'Messages & media', icon: 'message', group: 'app' },
  { id: 'notifications', name: 'Notifications', icon: 'bell', group: 'app' },
  { id: 'shortcuts', name: 'Shortcuts', icon: 'keyboard', group: 'app' },
  { id: 'emotes', name: 'Emotes', icon: 'smile', group: 'server' },
  { id: 'about', name: 'About', icon: 'info', group: 'server' },
] as const

export type SettingsSectionId = (typeof settingsSections)[number]['id']

/** Where a group's values are kept, shown beside its heading. */
export const scopeLabels = {
  device: 'This device',
  account: 'Synced to your account',
} as const

/** What the current user and device can reach right now. */
export interface SettingsAccess {
  /** Listed in IRCORD_EMOTE_MANAGERS. */
  emoteManager: boolean
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
  /** The heading the control sits under, when the section has several. */
  group?: string
  label: string
  /** The row's own heading, when the label adds context it lacks. */
  title?: string
  description?: string
  /** Other words for it: synonyms, older names, what it does. */
  keywords?: readonly string[]
  /**
   * Shown only when this holds. With `unavailable`, it is listed anyway with that explanation and
   * opens `fallback`, the control that makes it available; without, it is left out entirely.
   */
  requires?: Requirement
  unavailable?: string
  fallback?: string
}

export const settingsIndex: readonly SettingEntry[] = [
  // Appearance
  {
    id: 'appearance-sync',
    section: 'appearance',
    label: 'Same look on every device',
    description: 'Sync appearance to your account.',
    keywords: ['sync', 'devices', 'cloud', 'account'],
  },
  {
    id: 'theme-mode',
    section: 'appearance',
    group: 'Theme',
    label: 'Mode',
    description: 'System, dark or light.',
    keywords: ['dark mode', 'light mode', 'night mode', 'colour mode', 'color mode', 'theme'],
  },
  {
    id: 'colour-scheme',
    section: 'appearance',
    group: 'Theme',
    label: 'Colour scheme',
    keywords: ['color scheme', 'theme', 'palette', 'colours', 'colors'],
  },
  {
    id: 'scheme-variants',
    section: 'appearance',
    group: 'Theme',
    label: 'Dark and light variants',
    title: 'Dark variant',
    keywords: ['dark variant', 'light variant', 'theme', 'scheme'],
  },
  {
    id: 'accent',
    section: 'appearance',
    group: 'Theme',
    label: 'Accent',
    description: 'Selection, mentions and buttons.',
    keywords: ['accent colour', 'accent color', 'highlight colour', 'custom colour', 'color'],
  },
  {
    id: 'interface-font',
    section: 'appearance',
    group: 'Interface',
    label: 'Typeface',
    description: 'The font for menus, lists and dialogs.',
    // Typeface names (shared/appearance.ts interfaceFonts) so a search finds the picker by name.
    keywords: [
      'font',
      'interface font',
      'ui font',
      'geist',
      'inter',
      'ibm plex sans',
      'manrope',
      'nunito',
      'atkinson',
      'source sans',
    ],
  },
  {
    id: 'sidebar-density',
    section: 'appearance',
    group: 'Interface',
    label: 'Sidebar density',
    description: 'Row height in channel and member lists.',
    keywords: ['compact', 'comfortable', 'row height', 'channel list', 'member list'],
  },
  {
    id: 'corners',
    section: 'appearance',
    group: 'Interface',
    label: 'Corners',
    keywords: ['rounded', 'square', 'radius', 'border radius'],
  },
  {
    id: 'reduce-motion',
    section: 'appearance',
    group: 'Motion',
    label: 'Reduce motion',
    description: 'Transitions, animations and autoplay.',
    keywords: ['animation', 'accessibility', 'transitions', 'reduced motion'],
  },
  {
    id: 'appearance-reset',
    section: 'appearance',
    label: 'Reset appearance to defaults',
    title: 'Reset appearance',
    keywords: ['reset', 'defaults', 'restore'],
  },

  // Profile
  {
    id: 'profile-image',
    section: 'profile',
    label: 'Profile image',
    keywords: ['avatar', 'picture', 'photo', 'icon', 'upload'],
  },
  {
    id: 'display-name',
    section: 'profile',
    label: 'Display name',
    description: 'How you appear in IRCord.',
    keywords: ['name', 'real name', 'username shown'],
  },
  {
    id: 'name-colour',
    section: 'profile',
    label: 'Name color',
    description: 'Your own name’s colour.',
    keywords: ['name colour', 'nick colour', 'nick color', 'custom colour'],
  },
  {
    id: 'about-you',
    section: 'profile',
    label: 'About you',
    description: 'A line or two people see on your profile.',
    keywords: ['bio', 'description', 'status', 'profile text'],
  },

  // Networks
  {
    id: 'network-list',
    section: 'networks',
    label: 'Your networks',
    description: 'Edit, connect or disconnect an IRC network.',
    keywords: ['servers', 'connect', 'disconnect', 'reconnect', 'irc networks', 'edit network'],
  },
  {
    id: 'add-network',
    section: 'networks',
    label: 'Add a network',
    keywords: ['new network', 'add server', 'join server', 'connect'],
  },
  {
    id: 'register-channels',
    section: 'networks',
    group: 'Channels',
    label: 'Register channels I create',
    description: 'Registers new channels on the IRCord network to your account.',
    keywords: [
      'chanserv',
      'register channel',
      'channel owner',
      'founder',
      'claim channel',
      'keep channel',
    ],
  },
  {
    id: 'irc-links',
    section: 'networks',
    group: 'irc:// links',
    label: 'Open IRC links with IRCord',
    keywords: ['irc links', 'irc://', 'protocol handler', 'default client', 'links'],
  },

  // Messages & media
  {
    id: 'message-layout',
    section: 'messages',
    group: 'Layout',
    label: 'Message layout',
    description: 'Cozy, compact or IRC.',
    keywords: ['cozy', 'compact', 'irc style', 'one line', 'avatars', 'density'],
  },
  {
    id: 'message-font',
    section: 'messages',
    group: 'Layout',
    label: 'Message font',
    // Typeface names (shared/appearance.ts messageFonts) so a search finds the picker by name.
    keywords: [
      'font',
      'typeface',
      'text font',
      'chat font',
      'monospace',
      'serif',
      'atkinson',
      'lexend',
      'source serif',
      'literata',
      'jetbrains mono',
      'geist mono',
      'atkinson mono',
    ],
  },
  {
    id: 'text-size',
    section: 'messages',
    group: 'Layout',
    label: 'Text size',
    keywords: ['font size', 'message size', 'zoom', 'bigger', 'smaller', 'larger'],
  },
  {
    id: 'group-spacing',
    section: 'messages',
    group: 'Layout',
    label: 'Space between groups',
    description: 'Gap before each new speaker.',
    keywords: ['spacing', 'gap', 'padding', 'message spacing'],
  },
  {
    id: 'clock',
    section: 'messages',
    group: 'Layout',
    label: 'Clock',
    description: '24-hour or 12-hour time.',
    keywords: ['time format', 'timestamps', '12 hour', '24 hour', 'am pm'],
  },
  {
    id: 'coloured-names',
    section: 'messages',
    group: 'Layout',
    label: 'Coloured names',
    description: 'A stable colour for each person.',
    keywords: ['colored names', 'nick colours', 'nick colors', 'name colours'],
  },
  {
    id: 'channel-activity',
    section: 'messages',
    group: 'Conversation',
    label: 'Channel activity',
    description: 'Joins, parts and other small activity lines.',
    keywords: ['joins', 'parts', 'quits', 'events', 'join part messages', 'hide joins'],
  },
  {
    id: 'animate-media',
    section: 'messages',
    group: 'Animation',
    label: 'Play GIFs and animated emoji',
    description: 'Custom emoji and GIFs in the pickers.',
    keywords: ['animation', 'animated', 'gif', 'emoji', 'emote', 'still frame', 'motion', 'picker'],
  },
  {
    id: 'ignored-people',
    section: 'messages',
    group: 'Hidden messages',
    label: 'Ignored people',
    description: 'Their messages are hidden and never notify.',
    keywords: ['ignore', 'block', 'mute user', 'hostmask', 'mask'],
  },
  {
    id: 'messages-reset',
    section: 'messages',
    label: 'Reset message layout',
    keywords: ['reset', 'defaults', 'restore'],
  },

  // Notifications
  {
    id: 'install-app',
    section: 'notifications',
    group: 'This device',
    label: 'Install IRCord as an app',
    title: 'Add IRCord to your Home Screen',
    description: 'Its own window, an unread badge and more reliable notifications.',
    keywords: ['install', 'app', 'home screen', 'pwa', 'add to home screen'],
    requires: 'install',
  },
  {
    id: 'notifications-device',
    section: 'notifications',
    group: 'This device',
    label: 'Notifications on this device',
    description: 'Off, while open or push.',
    keywords: ['push', 'desktop notifications', 'browser notifications', 'alerts', 'devices'],
  },
  {
    id: 'notifications-test',
    section: 'notifications',
    group: 'This device',
    label: 'Send a test notification',
    title: 'Send test',
    keywords: ['test', 'try', 'push test'],
    requires: 'push',
    unavailable: 'Available when this device uses push notifications.',
    fallback: 'notifications-device',
  },
  {
    id: 'notification-sound',
    section: 'notifications',
    group: 'This device',
    label: 'Notification sound',
    description: 'A short chime while IRCord is open.',
    keywords: ['sound', 'chime', 'audio', 'ping', 'beep', 'volume'],
  },
  {
    id: 'notify-last-device',
    section: 'notifications',
    group: 'Across your devices',
    label: 'Notify the last-used device after',
    description: 'How long after you step away the device you used last is notified.',
    keywords: ['devices', 'phone', 'escalation', 'delay', 'inactive', 'idle', 'routing'],
  },
  {
    id: 'notify-all-devices',
    section: 'notifications',
    group: 'Across your devices',
    label: 'Notify all devices after',
    description: 'When every device is notified, with a reminder of what is unread.',
    keywords: ['devices', 'phone', 'tablet', 'reminder', 'missed', 'escalation', 'fan out'],
  },
  {
    id: 'notify-reset',
    section: 'notifications',
    group: 'Across your devices',
    label: 'Reset device timing',
    description: 'Back to the default notification delays.',
    keywords: ['defaults', 'reset', 'delays'],
  },
  {
    id: 'auto-away',
    section: 'notifications',
    group: 'Across your devices',
    label: 'Set me away when no device is connected',
    description: 'Mark yourself away on IRC after a while without IRCord open.',
    keywords: ['away', 'idle', 'presence', 'status', 'offline', 'auto away'],
  },
  {
    id: 'quiet-hours',
    section: 'notifications',
    group: 'Quiet hours',
    label: 'Quiet hours',
    description: 'Silence notifications between two times.',
    keywords: ['do not disturb', 'dnd', 'silence', 'mute', 'night', 'schedule', 'sleep'],
  },
  {
    id: 'quiet-hours-times',
    section: 'notifications',
    group: 'Quiet hours',
    label: 'Quiet hours start and end',
    title: 'From',
    keywords: ['from', 'until', 'times', 'schedule'],
    requires: 'quietHours',
    unavailable: 'Turn on quiet hours to set the times.',
    fallback: 'quiet-hours',
  },
  {
    id: 'channel-notifications',
    section: 'notifications',
    group: 'Notify me about',
    label: 'Channel notifications',
    description: 'Used where a channel’s level is Default.',
    keywords: ['channels', 'mentions', 'all messages', 'default level'],
  },
  {
    id: 'direct-notifications',
    section: 'notifications',
    group: 'Notify me about',
    label: 'Direct message notifications',
    keywords: ['direct messages', 'dm', 'private messages', 'pm', 'query', 'whispers'],
  },
  {
    id: 'highlight-words',
    section: 'notifications',
    group: 'Notify me about',
    label: 'Highlight words',
    description: 'Count as a mention, like your nick.',
    keywords: ['keywords', 'mentions', 'alert words', 'highlights', 'triggers'],
  },

  // Emotes
  {
    id: 'add-emote',
    section: 'emotes',
    label: 'Add an emote',
    keywords: ['upload emote', 'import', '7tv', 'frankerfacez', 'ffz', 'custom emoji'],
    requires: 'emoteManager',
  },
  {
    id: 'server-emotes',
    section: 'emotes',
    label: 'Server emotes',
    description: 'Custom emotes shared by everyone on this server.',
    keywords: ['emotes', 'emoji', 'custom emoji', 'catalog'],
  },

  // Account
  {
    id: 'password',
    section: 'account',
    group: 'Password',
    label: 'Change password',
    keywords: ['current password', 'new password', 'security', 'login'],
  },
  {
    id: 'log-out',
    section: 'account',
    group: 'Session',
    label: 'Log out',
    keywords: ['sign out', 'logout', 'session'],
  },
  {
    id: 'delete-account',
    section: 'account',
    group: 'Session',
    label: 'Delete account',
    description: 'Removes your networks, logins, history and settings.',
    keywords: ['remove account', 'close account', 'erase'],
  },

  // Shortcuts and About
  {
    id: 'keyboard-shortcuts',
    section: 'shortcuts',
    label: 'Keyboard shortcuts',
    keywords: ['hotkeys', 'keybindings', 'keys', 'shortcuts'],
  },
  {
    id: 'about-ircord',
    section: 'about',
    label: 'Version and changelog',
    title: 'Version',
    description: 'About IRCord, its version and what’s new.',
    keywords: ['about', 'version', 'release', 'changelog', 'what’s new', 'updates'],
  },
]

const byId = new Map(settingsIndex.map((entry) => [entry.id, entry]))

/** The entry with this id, if there is one. */
export const settingEntry = (id: string) => byId.get(id)

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

function rank(query: string[], phrase: string, entry: SettingEntry): number | null {
  const titles = [entry.label, entry.title ?? ''].map(settingWords)
  if (titles.some((t) => t.join(' ') === phrase)) return 3
  if (titles.some((t) => t.length && covers(query, t))) return 2
  const label = titles.flat()
  const keywords = (entry.keywords ?? []).map(settingWords)
  if (keywords.some((k) => k.join(' ') === phrase)) return 1.5
  const section = settingWords(sectionName(entry.section))
  const named = [...label, ...keywords.flat(), ...settingWords(entry.group ?? ''), ...section]
  if (covers(query, named)) return 1
  if (covers(query, [...named, ...settingWords(entry.description ?? '')])) return 0
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
  index: readonly SettingEntry[] = settingsIndex,
): { results: SettingResult[]; total: number } {
  const words = settingWords(query)
  if (!words.length) return { results: [], total: 0 }
  const phrase = words.join(' ')
  const results: (SettingResult & { order: number })[] = []
  index.forEach((entry, order) => {
    const missing = entry.requires && !access[entry.requires]
    if (missing && !entry.unavailable) return
    const score = rank(words, phrase, entry)
    if (score == null) return
    results.push({
      entry,
      rank: score,
      order,
      ...(missing ? { unavailable: entry.unavailable } : {}),
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

export function sectionName(id: SettingsSectionId): string {
  return settingsSections.find((s) => s.id === id)?.name ?? id
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
