// Appearance settings shared by the Nitro routes (validation) and the app (settings UI and <html>).
import { isHex } from './utils/color'
import { defaultSchemes, findScheme, schemeFor, schemeTokens, type SchemeMode } from './schemes'

/** Accent presets; any of them, or a custom colour, is adjusted to stay readable on every scheme. */
export const accentColors = {
  lavender: '#a78bfa',
  indigo: '#6366f1',
  blue: '#3b82f6',
  sky: '#0ea5e9',
  teal: '#14b8a6',
  emerald: '#10b981',
  lime: '#84cc16',
  amber: '#f59e0b',
  orange: '#f97316',
  rose: '#f43f5e',
  pink: '#ec4899',
  fuchsia: '#d946ef',
} as const
export type AccentPreset = keyof typeof accentColors
/** `scheme` uses the colour scheme's own accent; `custom` uses `accentCustom`. */
export type Accent = AccentPreset | 'scheme' | 'custom'

export const interfaceFonts = {
  geist: {
    label: 'Geist',
    note: 'Modern and precise',
    stack: "'Geist Variable', system-ui, sans-serif",
  },
  inter: {
    label: 'Inter',
    note: 'Crisp at small sizes',
    stack: "'Inter Variable', system-ui, sans-serif",
  },
  'ibm-plex': {
    label: 'IBM Plex Sans',
    note: 'Engineered and technical',
    stack: "'IBM Plex Sans Variable', system-ui, sans-serif",
  },
  manrope: {
    label: 'Manrope',
    note: 'Geometric and open',
    stack: "'Manrope Variable', system-ui, sans-serif",
  },
  nunito: {
    label: 'Nunito',
    note: 'Soft and rounded',
    stack: "'Nunito Variable', system-ui, sans-serif",
  },
  atkinson: {
    label: 'Atkinson',
    note: 'Designed for legibility',
    stack: "'Atkinson Hyperlegible Next Variable', system-ui, sans-serif",
  },
  'source-sans': {
    label: 'Source Sans',
    note: 'Humanist and familiar',
    stack: "'Source Sans 3 Variable', system-ui, sans-serif",
  },
  system: {
    label: 'System',
    note: 'Your device’s own font',
    stack: "system-ui, -apple-system, 'Segoe UI', roboto, sans-serif",
  },
} as const

/**
 * Typefaces for message text, grouped sans, serif, then monospace (a classic IRC look). The keys
 * are stored in account and device appearance, so they never change; the order is only display.
 */
export const messageFonts = {
  interface: { label: 'Interface', note: 'Same as the interface', stack: 'var(--font-ui)' },
  atkinson: {
    label: 'Atkinson',
    note: 'Clear letterforms',
    stack: "'Atkinson Hyperlegible Next Variable', system-ui, sans-serif",
  },
  lexend: {
    label: 'Lexend',
    note: 'Wide and easy to scan',
    stack: "'Lexend Variable', system-ui, sans-serif",
  },
  serif: {
    label: 'Source Serif',
    note: 'Calm, bookish serif',
    stack: "'Source Serif 4 Variable', georgia, serif",
  },
  literata: {
    label: 'Literata',
    note: 'Warm serif made for screens',
    stack: "'Literata Variable', georgia, serif",
  },
  'jetbrains-mono': {
    label: 'JetBrains Mono',
    note: 'Monospaced, terminal-like',
    stack: "'JetBrains Mono Variable', ui-monospace, monospace",
  },
  'geist-mono': {
    label: 'Geist Mono',
    note: 'Clean monospace',
    stack: "'Geist Mono Variable', ui-monospace, monospace",
  },
  'atkinson-mono': {
    label: 'Atkinson Mono',
    note: 'Unambiguous monospace',
    stack: "'Atkinson Hyperlegible Mono Variable', ui-monospace, monospace",
  },
} as const

export const appearanceOptions = {
  mode: ['system', 'dark', 'light'],
  font: Object.keys(interfaceFonts) as (keyof typeof interfaceFonts)[],
  messageFont: Object.keys(messageFonts) as (keyof typeof messageFonts)[],
  messageLayout: ['cozy', 'compact', 'irc'],
  density: ['comfortable', 'compact'],
  corners: ['rounded', 'soft', 'square'],
  timeFormat: ['24h', '12h'],
  motion: ['system', 'full', 'reduced'],
} as const
export const ranges = {
  messageSize: { min: 12, max: 20 },
  groupSpacing: { min: 0, max: 24 },
} as const

export type Appearance = {
  mode: (typeof appearanceOptions.mode)[number]
  schemeDark: string
  schemeLight: string
  accent: Accent
  accentCustom: string | null
  font: (typeof appearanceOptions.font)[number]
  messageFont: (typeof appearanceOptions.messageFont)[number]
  messageSize: number
  messageLayout: (typeof appearanceOptions.messageLayout)[number]
  /** Space between message groups, in pixels. */
  groupSpacing: number
  density: (typeof appearanceOptions.density)[number]
  corners: (typeof appearanceOptions.corners)[number]
  timeFormat: (typeof appearanceOptions.timeFormat)[number]
  coloredNames: boolean
  animateMedia: boolean
  motion: (typeof appearanceOptions.motion)[number]
}

/** The account copy as the server stores it. */
export interface StoredAppearance {
  settings: Appearance | null
  updatedAt: string | null
}

export const defaultAppearance: Appearance = {
  mode: 'dark',
  schemeDark: defaultSchemes.dark,
  schemeLight: defaultSchemes.light,
  accent: 'scheme',
  accentCustom: null,
  font: 'geist',
  messageFont: 'interface',
  messageSize: 14,
  messageLayout: 'cozy',
  groupSpacing: 12,
  density: 'comfortable',
  corners: 'rounded',
  timeFormat: '24h',
  coloredNames: true,
  animateMedia: true,
  motion: 'system',
}

/**
 * The appearance settings shown under Messages & media; the rest are under Appearance. Both
 * sections share one stored value and one sync switch, and each resets only its own keys.
 */
export const messageAppearanceKeys = [
  'messageLayout',
  'messageFont',
  'messageSize',
  'groupSpacing',
  'timeFormat',
  'coloredNames',
  'animateMedia',
] as const satisfies readonly (keyof Appearance)[]

/** The defaults for just these keys, to reset one section without touching the other. */
export function appearanceDefaults<K extends keyof Appearance>(keys: readonly K[]) {
  return Object.fromEntries(keys.map((k) => [k, defaultAppearance[k]])) as Pick<Appearance, K>
}
export const interfaceAppearanceKeys = (
  Object.keys(defaultAppearance) as (keyof Appearance)[]
).filter((k) => !(messageAppearanceKeys as readonly string[]).includes(k))

const pick = <T extends string>(value: unknown, allowed: readonly T[], fallback: T): T =>
  allowed.includes(value as T) ? (value as T) : fallback
const clampInt = (value: unknown, range: { min: number; max: number }, fallback: number) =>
  Number.isInteger(value) ? Math.max(range.min, Math.min(range.max, value as number)) : fallback

/** Accepts anything and returns a complete, valid appearance. Unknown keys are dropped. */
export function sanitizeAppearance(input: unknown): Appearance {
  const value = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const d = defaultAppearance
  const scheme = (id: unknown, mode: SchemeMode, fallback: string) =>
    findScheme(id)?.mode === mode ? (id as string) : fallback
  return {
    mode: pick(value.mode, appearanceOptions.mode, d.mode),
    schemeDark: scheme(value.schemeDark, 'dark', d.schemeDark),
    schemeLight: scheme(value.schemeLight, 'light', d.schemeLight),
    accent: pick(
      value.accent,
      ['scheme', 'custom', ...(Object.keys(accentColors) as AccentPreset[])],
      d.accent,
    ),
    accentCustom: isHex(value.accentCustom) ? value.accentCustom.toLowerCase() : null,
    font: pick(value.font, appearanceOptions.font, d.font),
    messageFont: pick(value.messageFont, appearanceOptions.messageFont, d.messageFont),
    messageSize: clampInt(value.messageSize, ranges.messageSize, d.messageSize),
    messageLayout: pick(value.messageLayout, appearanceOptions.messageLayout, d.messageLayout),
    groupSpacing: clampInt(value.groupSpacing, ranges.groupSpacing, d.groupSpacing),
    density: pick(value.density, appearanceOptions.density, d.density),
    corners: pick(value.corners, appearanceOptions.corners, d.corners),
    timeFormat: pick(value.timeFormat, appearanceOptions.timeFormat, d.timeFormat),
    coloredNames: typeof value.coloredNames === 'boolean' ? value.coloredNames : d.coloredNames,
    animateMedia: typeof value.animateMedia === 'boolean' ? value.animateMedia : d.animateMedia,
    motion: pick(value.motion, appearanceOptions.motion, d.motion),
  }
}

/** The colour accent tokens are derived from, or null to use the scheme's own accent. */
export function accentSource(appearance: Appearance): string | null {
  if (appearance.accent === 'scheme') return null
  if (appearance.accent === 'custom') return appearance.accentCustom
  return accentColors[appearance.accent]
}

const radii = { rounded: [8, 12], soft: [5, 8], square: [2, 3] } as const

/**
 * Everything the document needs for one appearance in one mode: a stylesheet of custom
 * properties and the attributes set on <html>. Cached locally so the boot script can apply it
 * before the app loads.
 */
export function compileAppearance(appearance: Appearance, mode: SchemeMode) {
  const scheme = schemeFor(mode === 'dark' ? appearance.schemeDark : appearance.schemeLight, mode)
  const tokens = schemeTokens(scheme, accentSource(appearance))
  const [radius, radiusLarge] = radii[appearance.corners]
  const declarations = [
    ...Object.entries(tokens).map(([name, value]) => `--${name}: ${value}`),
    `--font-ui: ${interfaceFonts[appearance.font].stack}`,
    `--font-message: ${messageFonts[appearance.messageFont].stack}`,
    `--message-size: ${appearance.messageSize}px`,
    `--group-gap: ${appearance.groupSpacing}px`,
    `--radius: ${radius}px`,
    `--radius-large: ${radiusLarge}px`,
  ]
  return {
    // Doubled :root outranks the stylesheet defaults wherever this <style> lands in <head>.
    css: `:root:root { ${declarations.join('; ')}; }`,
    attrs: {
      'data-theme': mode,
      'data-layout': appearance.messageLayout,
      'data-density': appearance.density,
      'data-corners': appearance.corners,
    } as Record<string, string>,
    background: tokens.main,
    tokens,
    scheme,
  }
}
