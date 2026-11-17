// Personal themes (#88): a named document the user owns, with optional colours, one background
// image and custom CSS. Shared by the Nitro routes (validation) and the app (studio, library and
// <html>). The CSS is filtered in the browser at apply time (shared/theme-css.ts); here it is
// only bounded text.
import { contrast, isHex, mix } from './utils/color'
import { findScheme, schemeFor, type PaletteColor, type Scheme, type SchemeMode } from './schemes'

/** The document format this client writes and understands. A newer one is never applied. */
export const themeFormat = 1
export const themeLimits = {
  themes: 20,
  name: 60,
  css: 64 * 1024,
  /** The whole library, as the server stores it. */
  document: 1 << 20,
  /** A file or pasted text offered for import. */
  importText: 256 * 1024,
} as const

/** Size in UTF-8 bytes, as the backend counts it; the limits above are bytes. */
export const byteLength = (text: string) => new TextEncoder().encode(text).length

export const seedKeys = ['canvas', 'surface', 'overlay', 'ink', 'muted', 'accent'] as const
export type SeedKey = (typeof seedKeys)[number]
export const paletteKeys = [
  'red',
  'orange',
  'yellow',
  'green',
  'cyan',
  'blue',
  'purple',
  'pink',
] as const satisfies readonly PaletteColor[]

/** The same seeds every built-in scheme is made of. */
export type ThemeSeeds = Record<SeedKey, string> & { palette: Record<PaletteColor, string> }

export const backgroundOptions = {
  placement: ['conversation', 'app'],
  fit: ['cover', 'contain', 'tile'],
  position: ['center', 'top', 'bottom', 'left', 'right'],
} as const
export const backgroundRanges = {
  /** Percent of the surface colour laid over the image. */
  dim: { min: 0, max: 90 },
  blur: { min: 0, max: 20 },
  /** Percent opacity of the sidebar and panels over an app-wide image. */
  panelOpacity: { min: 40, max: 100 },
} as const

export interface ThemeBackground {
  /** The upload's path on this instance, or null when an imported theme still needs an image. */
  url: string | null
  placement: (typeof backgroundOptions.placement)[number]
  fit: (typeof backgroundOptions.fit)[number]
  position: (typeof backgroundOptions.position)[number]
  dim: number
  blur: number
  panelOpacity: number
  /** The image's dark and light tones, measured when it was chosen; they set the contrast floors. */
  tones: [string, string] | null
}

export interface Theme {
  format: number
  id: string
  name: string
  base: { dark: string; light: string }
  colors: Partial<Record<SchemeMode, ThemeSeeds>>
  background: ThemeBackground | null
  css: string
  /** The IRCord version that last saved it. */
  madeWith: string
  updatedAt: string
}

/** The account's `themes` document. It is account data whatever the appearance sync switch says. */
export interface ThemeLibrary {
  format: number
  /** The account's own "Use custom CSS" switch. */
  customCss: boolean
  themes: Theme[]
  updatedAt: string | null
}

export const emptyThemeLibrary = (): ThemeLibrary => ({
  format: themeFormat,
  customCss: true,
  themes: [],
  updatedAt: null,
})

const idPattern = /^[A-Za-z0-9_-]{1,40}$/
/**
 * Only uploads on this instance: `/files/<id>/<token>/<name>` (Go backend) or the demo's
 * `/api/v1/uploads/<id>`. The character set excludes quotes, brackets, backslashes and spaces, so
 * the path is safe inside `url("…")`.
 */
const filePath =
  /^\/(?:files\/[A-Za-z0-9_-]{1,80}\/[A-Za-z0-9_-]{1,200}\/[A-Za-z0-9._~%$&+,:;=@-]{1,400}|api\/v1\/uploads\/[A-Za-z0-9-]{1,80})$/

export const isThemeId = (value: unknown): value is string =>
  typeof value === 'string' && idPattern.test(value)

/** The path of an upload on this instance, from the address the upload endpoint returned. */
export function instanceFilePath(address: unknown, origin?: string): string | null {
  if (typeof address !== 'string') return null
  try {
    const url = new URL(address, origin ?? 'http://instance.invalid')
    if (url.search || url.hash) return null
    return filePath.test(url.pathname) ? url.pathname : null
  } catch {
    return null
  }
}

export function newThemeId() {
  const bytes = new Uint8Array(9)
  crypto.getRandomValues(bytes)
  return 't-' + Array.from(bytes, (b) => b.toString(36).padStart(2, '0')).join('')
}

const pick = <T extends string>(value: unknown, allowed: readonly T[], fallback: T): T =>
  allowed.includes(value as T) ? (value as T) : fallback
const clamp = (value: unknown, range: { min: number; max: number }, fallback: number) =>
  typeof value === 'number' && Number.isFinite(value)
    ? Math.round(Math.max(range.min, Math.min(range.max, value)))
    : fallback
const text = (value: unknown, max: number) => (typeof value === 'string' ? value.slice(0, max) : '')
const record = (value: unknown) =>
  (value && typeof value === 'object' && !Array.isArray(value) ? value : {}) as Record<
    string,
    unknown
  >

/** Seeds from a scheme, the starting point when a mode is first edited. */
export function seedsOf(scheme: Scheme): ThemeSeeds {
  return {
    canvas: scheme.canvas,
    surface: scheme.surface,
    overlay: scheme.overlay,
    ink: scheme.ink,
    muted: scheme.muted,
    accent: scheme.accent,
    palette: { ...scheme.palette },
  }
}

function sanitizeSeeds(input: unknown): ThemeSeeds | undefined {
  const value = record(input)
  const palette = record(value.palette)
  if (!seedKeys.every((key) => isHex(value[key])) || !paletteKeys.every((k) => isHex(palette[k])))
    return undefined
  const lower = (hex: unknown) => (hex as string).toLowerCase()
  return {
    ...(Object.fromEntries(seedKeys.map((key) => [key, lower(value[key])])) as Record<
      SeedKey,
      string
    >),
    palette: Object.fromEntries(paletteKeys.map((key) => [key, lower(palette[key])])) as Record<
      PaletteColor,
      string
    >,
  }
}

export const defaultBackground = (): ThemeBackground => ({
  url: null,
  placement: 'conversation',
  fit: 'cover',
  position: 'center',
  dim: 55,
  blur: 0,
  panelOpacity: 85,
  tones: null,
})

function sanitizeBackground(input: unknown): ThemeBackground | null {
  if (!input || typeof input !== 'object') return null
  const value = record(input)
  const d = defaultBackground()
  const tones: unknown[] = Array.isArray(value.tones) ? value.tones : []
  return {
    url: instanceFilePath(value.url),
    placement: pick(value.placement, backgroundOptions.placement, d.placement),
    fit: pick(value.fit, backgroundOptions.fit, d.fit),
    position: pick(value.position, backgroundOptions.position, d.position),
    dim: clamp(value.dim, backgroundRanges.dim, d.dim),
    blur: clamp(value.blur, backgroundRanges.blur, d.blur),
    panelOpacity: clamp(value.panelOpacity, backgroundRanges.panelOpacity, d.panelOpacity),
    tones:
      tones.length === 2 && tones.every(isHex)
        ? [(tones[0] as string).toLowerCase(), (tones[1] as string).toLowerCase()]
        : null,
  }
}

/** Accepts anything; returns a valid theme, or null without a usable id. */
export function sanitizeTheme(input: unknown): Theme | null {
  const value = record(input)
  if (!isThemeId(value.id)) return null
  const base = record(value.base)
  const scheme = (id: unknown, mode: SchemeMode) =>
    findScheme(id)?.mode === mode ? (id as string) : schemeFor(id, mode).id
  const colors = record(value.colors)
  const dark = sanitizeSeeds(colors.dark)
  const light = sanitizeSeeds(colors.light)
  const format = value.format
  return {
    format: Number.isInteger(format) && (format as number) >= 1 ? (format as number) : themeFormat,
    id: value.id,
    name: text(value.name, themeLimits.name).trim() || 'Untitled theme',
    base: { dark: scheme(base.dark, 'dark'), light: scheme(base.light, 'light') },
    colors: { ...(dark && { dark }), ...(light && { light }) },
    background: sanitizeBackground(value.background),
    css: text(value.css, themeLimits.css),
    madeWith: text(value.madeWith, 40),
    updatedAt: text(value.updatedAt, 40),
  }
}

export function sanitizeThemeLibrary(input: unknown): ThemeLibrary {
  const value = record(input)
  const seen = new Set<string>()
  const themes: Theme[] = []
  for (const item of Array.isArray(value.themes) ? value.themes : []) {
    const theme = sanitizeTheme(item)
    if (!theme || seen.has(theme.id) || themes.length >= themeLimits.themes) continue
    seen.add(theme.id)
    themes.push(theme)
  }
  return {
    format: themeFormat,
    customCss: value.customCss !== false,
    themes,
    updatedAt: typeof value.updatedAt === 'string' ? value.updatedAt.slice(0, 40) : null,
  }
}

/** Whether this client can apply the theme. */
export const themeSupported = (theme: Theme) => theme.format <= themeFormat

/** The scheme a theme draws in one mode: its base, with the theme's own seeds when it has them. */
export function themeScheme(theme: Theme, mode: SchemeMode): Scheme {
  const base = schemeFor(theme.base[mode], mode)
  const seeds = theme.colors[mode]
  // The base's exact token overrides belong to its own seeds, so they go with them.
  return seeds ? { ...base, ...seeds, palette: { ...seeds.palette }, tokens: undefined } : base
}

// ---- Background ---------------------------------------------------------------------------------

/**
 * The lowest dim and panel opacity that keep text readable over the image: message text at 4.5:1
 * and quieter text at 3:1 on both of the image's measured tones. Unknown tones get a safe guess.
 */
export function backgroundFloors(
  tokens: { main: string; sidebar: string; text: string; body: string; muted: string },
  tones: [string, string] | null,
) {
  if (!tones) return { dim: 60, panelOpacity: 85 }
  const readable = (surface: string) =>
    contrast(tokens.text, surface) >= 4.5 &&
    contrast(tokens.body, surface) >= 4.5 &&
    contrast(tokens.muted, surface) >= 3
  const lowest = (range: { min: number; max: number }, cover: string) => {
    for (let percent = range.min; percent <= range.max; percent += 5)
      if (tones.every((tone) => readable(mix(tone, cover, percent / 100)))) return percent
    return range.max
  }
  return {
    dim: lowest(backgroundRanges.dim, tokens.main),
    panelOpacity: lowest(backgroundRanges.panelOpacity, tokens.sidebar),
  }
}

const sizes = { cover: 'cover', contain: 'contain', tile: 'auto' } as const

/** Custom properties the app's fixed background CSS reads (workspace-layout.css). */
export function backgroundDeclarations(
  background: ThemeBackground,
  tokens: Parameters<typeof backgroundFloors>[0],
) {
  if (!background.url) return []
  const floors = backgroundFloors(tokens, background.tones)
  const dim = Math.max(background.dim, floors.dim) / 100
  const panel = Math.max(background.panelOpacity, floors.panelOpacity) / 100
  return [
    `--theme-image: url("${background.url}")`,
    `--theme-size: ${sizes[background.fit]}`,
    `--theme-repeat: ${background.fit === 'tile' ? 'repeat' : 'no-repeat'}`,
    `--theme-position: ${background.position}`,
    `--theme-dim: ${dim}`,
    `--theme-blur: ${background.blur}px`,
    `--theme-panel: ${panel}`,
  ]
}

/** Dark and light tones of an image: the 10th and 90th percentile of its pixels by luminance. */
export function measureTones(pixels: Uint8ClampedArray): [string, string] | null {
  const samples: { l: number; hex: string }[] = []
  for (let i = 0; i + 3 < pixels.length; i += 4) {
    if (pixels[i + 3]! < 16) continue
    const [r, g, b] = [pixels[i]!, pixels[i + 1]!, pixels[i + 2]!]
    samples.push({
      l: 0.2126 * r + 0.7152 * g + 0.0722 * b,
      hex: '#' + [r, g, b].map((v) => v.toString(16).padStart(2, '0')).join(''),
    })
  }
  if (!samples.length) return null
  samples.sort((a, b) => a.l - b.l)
  const at = (q: number) => samples[Math.min(samples.length - 1, Math.floor(q * samples.length))]!
  return [at(0.1).hex, at(0.9).hex]
}

// ---- Import and export --------------------------------------------------------------------------

/** What an exported `.ircord-theme.json` file holds. The image itself is never included. */
export interface ThemeFile {
  ircordTheme: number
  name: string
  base: Theme['base']
  colors: Theme['colors']
  background: Omit<ThemeBackground, 'url' | 'tones'> | null
  css: string
  madeWith: string
}

export function exportTheme(theme: Theme): ThemeFile {
  const background = theme.background
    ? (({ url: _url, tones: _tones, ...settings }) => settings)(theme.background)
    : null
  return {
    ircordTheme: theme.format,
    name: theme.name,
    base: theme.base,
    colors: theme.colors,
    background,
    css: theme.css,
    madeWith: theme.madeWith,
  }
}

export const themeFileName = (theme: Theme) =>
  `${
    theme.name
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-|-$/g, '') || 'theme'
  }.ircord-theme.json`

export type ImportResult =
  | { ok: true; theme: Theme }
  | { ok: false; reason: 'too-large' | 'not-json' | 'not-theme' | 'newer' | 'css-too-long' }

/** Reads a file or pasted text. The result is a new, unapplied library entry with its own id. */
export function importTheme(raw: string, madeWith: string, now = new Date()): ImportResult {
  if (byteLength(raw) > themeLimits.importText) return { ok: false, reason: 'too-large' }
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return { ok: false, reason: 'not-json' }
  }
  const value = record(parsed)
  if (!Number.isInteger(value.ircordTheme) || (value.ircordTheme as number) < 1)
    return { ok: false, reason: 'not-theme' }
  if ((value.ircordTheme as number) > themeFormat) return { ok: false, reason: 'newer' }
  if (typeof value.css === 'string' && byteLength(value.css) > themeLimits.css)
    return { ok: false, reason: 'css-too-long' }
  const theme = sanitizeTheme({
    ...value,
    format: value.ircordTheme,
    id: newThemeId(),
    // An imported background never carries an address: the image has to be chosen here.
    background: value.background ? { ...record(value.background), url: null, tones: null } : null,
    madeWith: text(value.madeWith, 40) || madeWith,
    updatedAt: now.toISOString(),
  })
  return theme ? { ok: true, theme } : { ok: false, reason: 'not-theme' }
}

/** Whether `version` (a release such as v0.6.1) is newer than `than`. Unknown versions are not. */
export function newerVersion(version: string, than: string) {
  const parse = (v: string) => /^v?(\d+)\.(\d+)\.(\d+)/.exec(v)?.slice(1).map(Number)
  const a = parse(version)
  const b = parse(than)
  if (!a || !b) return false
  for (let i = 0; i < 3; i++) if (a[i] !== b[i]) return a[i]! > b[i]!
  return false
}
