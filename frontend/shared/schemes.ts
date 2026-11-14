// Colour schemes: published palettes reduced to a handful of seeds. Every interface token is
// derived from these (see schemeTokens), so any accent works on any scheme. Palette values come
// from the linked sources; WCAG notes mark the few values nudged for text contrast.
import { contrast, inkFor, isHex, mix, readableOn, readableOnAll } from './utils/color'

export type SchemeMode = 'dark' | 'light'

/** The eight palette hues every scheme provides; status colours map onto them. */
export type PaletteColor =
  'red' | 'orange' | 'yellow' | 'green' | 'cyan' | 'blue' | 'purple' | 'pink'

export interface Scheme {
  id: string
  family: string
  name: string
  mode: SchemeMode
  source: string
  /** Darkest chrome: the network rail and self panel. */
  canvas: string
  /** The conversation background. */
  surface: string
  /** Raised elements: hover, borders and controls are mixed from this. */
  overlay: string
  ink: string
  muted: string
  accent: string
  palette: Record<PaletteColor, string>
  /** Exact token values that replace the derived ones (IRCord's own tuned palette). */
  tokens?: Partial<Record<ThemeToken, string>>
}

export interface SchemeFamily {
  id: string
  name: string
  description: string
  variants: Scheme[]
}

const sources = {
  catppuccin: 'https://catppuccin.com/palette/',
  tokyo: 'https://github.com/folke/tokyonight.nvim',
  nord: 'https://www.nordtheme.com/docs/colors-and-palettes',
  rose: 'https://rosepinetheme.com/palette/',
  gruvbox: 'https://github.com/morhetz/gruvbox',
  dracula: 'https://draculatheme.com/contribute',
  solarized: 'https://ethanschoonover.com/solarized/',
  everforest: 'https://github.com/sainnhe/everforest/blob/master/palette.md',
  github: 'https://primer.style/foundations/color',
  kanagawa: 'https://github.com/rebelot/kanagawa.nvim',
}

const families: SchemeFamily[] = [
  {
    id: 'ircord',
    name: 'IRCord',
    description: 'Charcoal and lavender, the house style.',
    variants: [
      {
        id: 'ircord-dark',
        family: 'ircord',
        name: 'Charcoal',
        mode: 'dark',
        source: '',
        canvas: '#141418',
        surface: '#232329',
        overlay: '#3a3a44',
        ink: '#eeeef2',
        muted: '#a2a2af',
        accent: '#bfadf9',
        palette: {
          red: '#f2a4a6',
          orange: '#e8ad89',
          yellow: '#e5c38f',
          green: '#a7d9bf',
          cyan: '#91cbd1',
          blue: '#98b8f0',
          purple: '#bfadf9',
          pink: '#e7a1bc',
        },
        tokens: {
          rail: '#141418',
          sidebar: '#1b1b21',
          main: '#232329',
          panel: '#1d1d23',
          raised: '#2b2b33',
          hover: '#303039',
          input: '#2c2c34',
          border: '#34343d',
          text: '#eeeef2',
          body: '#ceced7',
          muted: '#a2a2af',
          faint: '#8f8c9c',
        },
      },
      {
        id: 'ircord-light',
        family: 'ircord',
        name: 'Daylight',
        mode: 'light',
        source: '',
        canvas: '#e8e6ef',
        surface: '#ffffff',
        overlay: '#dedbe7',
        ink: '#2c2935',
        muted: '#6e667c',
        accent: '#7350b9',
        palette: {
          red: '#a13b4a',
          orange: '#945c36',
          yellow: '#84692c',
          green: '#397752',
          cyan: '#307177',
          blue: '#3e649d',
          purple: '#7350b9',
          pink: '#984263',
        },
        tokens: {
          rail: '#e8e6ef',
          sidebar: '#f2f1f6',
          main: '#ffffff',
          panel: '#f8f7fb',
          raised: '#f0eef5',
          hover: '#e8e5ef',
          input: '#f6f5f9',
          border: '#dedbe7',
          text: '#2c2935',
          body: '#484351',
          muted: '#6e667c',
          faint: '#766e83',
        },
      },
    ],
  },
  {
    id: 'hibiki',
    name: 'Hibiki',
    description: 'Warm obsidian and coral with an ambient top glow.',
    variants: [
      {
        id: 'hibiki-dark',
        family: 'hibiki',
        name: 'Stage',
        mode: 'dark',
        source: '',
        canvas: '#111013',
        surface: '#18161a',
        overlay: '#2d2730',
        ink: '#f4f0f3',
        muted: '#a89ea6',
        accent: '#f43f5e',
        palette: {
          red: '#f87171',
          orange: '#fb923c',
          yellow: '#fbbf24',
          green: '#4ade80',
          cyan: '#38bdf8',
          blue: '#818cf8',
          purple: '#c084fc',
          pink: '#fb7185',
        },
      },
      {
        id: 'hibiki-light',
        family: 'hibiki',
        name: 'Paper',
        mode: 'light',
        source: '',
        canvas: '#efe9ec',
        surface: '#fcf9fa',
        overlay: '#dfd6db',
        ink: '#261f24',
        muted: '#6b5e66',
        accent: '#d92648',
        palette: {
          red: '#be123c',
          orange: '#c2410c',
          yellow: '#a16207',
          green: '#15803d',
          cyan: '#0369a1',
          blue: '#4338ca',
          purple: '#7e22ce',
          pink: '#be185d',
        },
      },
    ],
  },
  {
    id: 'midnight',
    name: 'Midnight',
    description: 'True black for OLED screens, with crisp contrast.',
    variants: [
      {
        id: 'midnight',
        family: 'midnight',
        name: 'OLED',
        mode: 'dark',
        source: '',
        canvas: '#000000',
        surface: '#0b0b0c',
        overlay: '#26262a',
        ink: '#f5f5f7',
        muted: '#a1a1a8',
        accent: '#5eead4',
        palette: {
          red: '#ff6b6b',
          orange: '#ffa94d',
          yellow: '#ffd43b',
          green: '#69db7c',
          cyan: '#66d9e8',
          blue: '#74c0fc',
          purple: '#b197fc',
          pink: '#f783ac',
        },
      },
    ],
  },
  {
    id: 'catppuccin',
    name: 'Catppuccin',
    description: 'Soothing pastels with gentle, low-contrast surfaces.',
    variants: [
      {
        id: 'catppuccin-mocha',
        family: 'catppuccin',
        name: 'Mocha',
        mode: 'dark',
        source: sources.catppuccin,
        canvas: '#181825',
        surface: '#1e1e2e',
        overlay: '#313244',
        ink: '#cdd6f4',
        muted: '#a6adc8',
        accent: '#cba6f7',
        palette: {
          red: '#f38ba8',
          orange: '#fab387',
          yellow: '#f9e2af',
          green: '#a6e3a1',
          cyan: '#89dceb',
          blue: '#89b4fa',
          purple: '#cba6f7',
          pink: '#f5c2e7',
        },
      },
      {
        id: 'catppuccin-macchiato',
        family: 'catppuccin',
        name: 'Macchiato',
        mode: 'dark',
        source: sources.catppuccin,
        canvas: '#1e2030',
        surface: '#24273a',
        overlay: '#363a4f',
        ink: '#cad3f5',
        muted: '#a5adcb',
        accent: '#c6a0f6',
        palette: {
          red: '#ed8796',
          orange: '#f5a97f',
          yellow: '#eed49f',
          green: '#a6da95',
          cyan: '#91d7e3',
          blue: '#8aadf4',
          purple: '#c6a0f6',
          pink: '#f5bde6',
        },
      },
      {
        id: 'catppuccin-frappe',
        family: 'catppuccin',
        name: 'Frappé',
        mode: 'dark',
        source: sources.catppuccin,
        canvas: '#292c3c',
        surface: '#303446',
        overlay: '#414559',
        ink: '#c6d0f5',
        muted: '#a5adce',
        accent: '#ca9ee6',
        palette: {
          red: '#e78284',
          orange: '#ef9f76',
          yellow: '#e5c890',
          green: '#a6d189',
          cyan: '#99d1db',
          blue: '#8caaee',
          purple: '#ca9ee6',
          pink: '#f4b8e4',
        },
      },
      {
        id: 'catppuccin-latte',
        family: 'catppuccin',
        name: 'Latte',
        mode: 'light',
        source: sources.catppuccin,
        canvas: '#e6e9ef',
        surface: '#eff1f5',
        overlay: '#ccd0da',
        ink: '#4c4f69',
        muted: '#65687d', // WCAG: was #6c6f85 (4.06:1), lightness nudged to 4.51:1
        accent: '#8839ef',
        palette: {
          red: '#d20f39',
          orange: '#fe640b',
          yellow: '#df8e1d',
          green: '#40a02b',
          cyan: '#04a5e5',
          blue: '#1e66f5',
          purple: '#8839ef',
          pink: '#ea76cb',
        },
      },
    ],
  },
  {
    id: 'tokyo-night',
    name: 'Tokyo Night',
    description: 'Neon city lights over deep indigo.',
    variants: [
      {
        id: 'tokyo-night',
        family: 'tokyo-night',
        name: 'Night',
        mode: 'dark',
        source: sources.tokyo,
        canvas: '#16161e',
        surface: '#1a1b26',
        overlay: '#292e42',
        ink: '#c0caf5',
        muted: '#a9b1d6',
        accent: '#7aa2f7',
        palette: {
          red: '#f7768e',
          orange: '#ff9e64',
          yellow: '#e0af68',
          green: '#9ece6a',
          cyan: '#7dcfff',
          blue: '#7aa2f7',
          purple: '#bb9af7',
          pink: '#ff7eb6',
        },
      },
      {
        id: 'tokyo-storm',
        family: 'tokyo-night',
        name: 'Storm',
        mode: 'dark',
        source: sources.tokyo,
        canvas: '#1f2335',
        surface: '#24283b',
        overlay: '#3b4261',
        ink: '#c0caf5',
        muted: '#a9b1d6',
        accent: '#7aa2f7',
        palette: {
          red: '#f7768e',
          orange: '#ff9e64',
          yellow: '#e0af68',
          green: '#9ece6a',
          cyan: '#7dcfff',
          blue: '#7aa2f7',
          purple: '#bb9af7',
          pink: '#ff7eb6',
        },
      },
      {
        id: 'tokyo-moon',
        family: 'tokyo-night',
        name: 'Moon',
        mode: 'dark',
        source: sources.tokyo,
        canvas: '#1e2030',
        surface: '#222436',
        overlay: '#2f334d',
        ink: '#c8d3f5',
        muted: '#a9b8e8',
        accent: '#82aaff',
        palette: {
          red: '#ff757f',
          orange: '#ff966c',
          yellow: '#ffc777',
          green: '#c3e88d',
          cyan: '#86e1fc',
          blue: '#82aaff',
          purple: '#c099ff',
          pink: '#fca7ea',
        },
      },
      {
        id: 'tokyo-day',
        family: 'tokyo-night',
        name: 'Day',
        mode: 'light',
        source: sources.tokyo,
        canvas: '#d0d5e3',
        surface: '#e1e2e7',
        overlay: '#c4c8da',
        ink: '#3258af', // WCAG: was #3760bf (3.99:1), lightness nudged to 4.54:1
        muted: '#4a5993', // WCAG: was #6172b0 (3.15:1), lightness nudged to 4.55:1
        accent: '#2e7de9',
        palette: {
          red: '#f52a65',
          orange: '#b15c00',
          yellow: '#8c6c3e',
          green: '#587539',
          cyan: '#007197',
          blue: '#2e7de9',
          purple: '#9854f1',
          pink: '#d20065',
        },
      },
    ],
  },
  {
    id: 'nord',
    name: 'Nord',
    description: 'An arctic, north-bluish palette.',
    variants: [
      {
        id: 'nord-dark',
        family: 'nord',
        name: 'Polar Night',
        mode: 'dark',
        source: sources.nord,
        canvas: '#2e3440',
        surface: '#3b4252',
        overlay: '#4c566a',
        ink: '#eceff4',
        muted: '#d8dee9',
        accent: '#88c0d0',
        palette: {
          red: '#bf616a',
          orange: '#d08770',
          yellow: '#ebcb8b',
          green: '#a3be8c',
          cyan: '#8fbcbb',
          blue: '#81a1c1',
          purple: '#b48ead',
          pink: '#c895bf',
        },
      },
      {
        id: 'nord-light',
        family: 'nord',
        name: 'Snow Storm',
        mode: 'light',
        source: sources.nord,
        canvas: '#e5e9f0',
        surface: '#eceff4',
        overlay: '#d8dee9',
        ink: '#2e3440',
        muted: '#4c566a',
        accent: '#5e81ac',
        palette: {
          red: '#a3434d',
          orange: '#a85d43',
          yellow: '#8a6d1f',
          green: '#5d7a45',
          cyan: '#3b7a79',
          blue: '#5e81ac',
          purple: '#85667f',
          pink: '#9a5a8c',
        },
      },
    ],
  },
  {
    id: 'rose-pine',
    name: 'Rosé Pine',
    description: 'All natural pine, faux fur and a bit of soho vibes.',
    variants: [
      {
        id: 'rose-pine',
        family: 'rose-pine',
        name: 'Main',
        mode: 'dark',
        source: sources.rose,
        canvas: '#191724',
        surface: '#1f1d2e',
        overlay: '#26233a',
        ink: '#e0def4',
        muted: '#908caa',
        accent: '#ebbcba',
        palette: {
          red: '#eb6f92',
          orange: '#ea9a97',
          yellow: '#f6c177',
          green: '#9ccfd8',
          cyan: '#9ccfd8',
          blue: '#31748f',
          purple: '#c4a7e7',
          pink: '#ebbcba',
        },
      },
      {
        id: 'rose-pine-moon',
        family: 'rose-pine',
        name: 'Moon',
        mode: 'dark',
        source: sources.rose,
        canvas: '#232136',
        surface: '#2a273f',
        overlay: '#393552',
        ink: '#e0def4',
        muted: '#918dab', // WCAG: was #908caa (4.46:1), lightness nudged to 4.52:1
        accent: '#ea9a97',
        palette: {
          red: '#eb6f92',
          orange: '#ea9a97',
          yellow: '#f6c177',
          green: '#9ccfd8',
          cyan: '#9ccfd8',
          blue: '#3e8fb0',
          purple: '#c4a7e7',
          pink: '#ea9a97',
        },
      },
      {
        id: 'rose-pine-dawn',
        family: 'rose-pine',
        name: 'Dawn',
        mode: 'light',
        source: sources.rose,
        canvas: '#f2e9e1',
        surface: '#fffaf3',
        overlay: '#dfdad9',
        ink: '#575279',
        muted: '#6a6683', // WCAG: was #797593 (3.67:1), lightness nudged to 4.56:1
        accent: '#d47873', // WCAG: was #d7827e (2.74:1), lightness nudged to 3.00:1
        palette: {
          red: '#b4637a',
          orange: '#d7827e',
          yellow: '#ea9d34',
          green: '#56949f',
          cyan: '#56949f',
          blue: '#286983',
          purple: '#907aa9',
          pink: '#d7827e',
        },
      },
    ],
  },
  {
    id: 'gruvbox',
    name: 'Gruvbox',
    description: 'Retro groove with warm, earthy contrast.',
    variants: [
      {
        id: 'gruvbox-dark',
        family: 'gruvbox',
        name: 'Dark',
        mode: 'dark',
        source: sources.gruvbox,
        canvas: '#1d2021',
        surface: '#282828',
        overlay: '#3c3836',
        ink: '#ebdbb2',
        muted: '#bdae93',
        accent: '#fabd2f',
        palette: {
          red: '#fb4934',
          orange: '#fe8019',
          yellow: '#fabd2f',
          green: '#b8bb26',
          cyan: '#8ec07c',
          blue: '#83a598',
          purple: '#d3869b',
          pink: '#d3869b',
        },
      },
      {
        id: 'gruvbox-light',
        family: 'gruvbox',
        name: 'Light',
        mode: 'light',
        source: sources.gruvbox,
        canvas: '#f2e5bc',
        surface: '#fbf1c7',
        overlay: '#d5c4a1',
        ink: '#3c3836',
        muted: '#665c54',
        accent: '#af3a03',
        palette: {
          red: '#9d0006',
          orange: '#af3a03',
          yellow: '#b57614',
          green: '#79740e',
          cyan: '#427b58',
          blue: '#076678',
          purple: '#8f3f71',
          pink: '#8f3f71',
        },
      },
    ],
  },
  {
    id: 'dracula',
    name: 'Dracula',
    description: 'Dark, vivid and a little theatrical.',
    variants: [
      {
        id: 'dracula',
        family: 'dracula',
        name: 'Dracula',
        mode: 'dark',
        source: sources.dracula,
        canvas: '#21222c',
        surface: '#282a36',
        overlay: '#44475a',
        ink: '#f8f8f2',
        muted: '#a3abd1',
        accent: '#bd93f9',
        palette: {
          red: '#ff5555',
          orange: '#ffb86c',
          yellow: '#f1fa8c',
          green: '#50fa7b',
          cyan: '#8be9fd',
          blue: '#8aa0f2',
          purple: '#bd93f9',
          pink: '#ff79c6',
        },
      },
      {
        id: 'alucard',
        family: 'dracula',
        name: 'Alucard',
        mode: 'light',
        source: sources.dracula,
        canvas: '#f5f1de',
        surface: '#fffbeb',
        overlay: '#dedccf',
        ink: '#1f1f1f',
        muted: '#635d97',
        accent: '#644ac9',
        palette: {
          red: '#cb3a2a',
          orange: '#a34d14',
          yellow: '#846e15',
          green: '#14710a',
          cyan: '#036a96',
          blue: '#3a56b8',
          purple: '#644ac9',
          pink: '#a3144d',
        },
      },
    ],
  },
  {
    id: 'solarized',
    name: 'Solarized',
    description: 'Precision colours tuned for readability.',
    variants: [
      {
        id: 'solarized-dark',
        family: 'solarized',
        name: 'Dark',
        mode: 'dark',
        source: sources.solarized,
        canvas: '#002b36',
        surface: '#073642',
        overlay: '#0e4a59',
        ink: '#eee8d5',
        muted: '#93a1a1',
        accent: '#268bd2',
        palette: {
          red: '#dc322f',
          orange: '#cb4b16',
          yellow: '#b58900',
          green: '#859900',
          cyan: '#2aa198',
          blue: '#268bd2',
          purple: '#6c71c4',
          pink: '#d33682',
        },
      },
      {
        id: 'solarized-light',
        family: 'solarized',
        name: 'Light',
        mode: 'light',
        source: sources.solarized,
        canvas: '#eee8d5',
        surface: '#fdf6e3',
        overlay: '#ddd6c1',
        ink: '#073642',
        muted: '#566c73', // WCAG: was #586e75 (4.39:1), lightness nudged to 4.52:1
        accent: '#268bd2',
        palette: {
          red: '#dc322f',
          orange: '#cb4b16',
          yellow: '#b58900',
          green: '#859900',
          cyan: '#2aa198',
          blue: '#268bd2',
          purple: '#6c71c4',
          pink: '#d33682',
        },
      },
    ],
  },
  {
    id: 'everforest',
    name: 'Everforest',
    description: 'Green-based, warm and easy on the eyes.',
    variants: [
      {
        id: 'everforest-dark',
        family: 'everforest',
        name: 'Dark',
        mode: 'dark',
        source: sources.everforest,
        canvas: '#232a2e',
        surface: '#2d353b',
        overlay: '#3d484d',
        ink: '#d3c6aa',
        muted: '#9da9a0',
        accent: '#a7c080',
        palette: {
          red: '#e67e80',
          orange: '#e69875',
          yellow: '#dbbc7f',
          green: '#a7c080',
          cyan: '#83c092',
          blue: '#7fbbb3',
          purple: '#d699b6',
          pink: '#d699b6',
        },
      },
      {
        id: 'everforest-light',
        family: 'everforest',
        name: 'Light',
        mode: 'light',
        source: sources.everforest,
        canvas: '#efebd4',
        surface: '#fdf6e3',
        overlay: '#e0dcc7',
        ink: '#5c6a72',
        muted: '#606d5f', // WCAG: was #829181 (2.77:1), lightness nudged to 4.55:1
        accent: '#859801', // WCAG: was #8da101 (2.69:1), lightness nudged to 3.00:1
        palette: {
          red: '#f85552',
          orange: '#f57d26',
          yellow: '#dfa000',
          green: '#8da101',
          cyan: '#35a77c',
          blue: '#3a94c5',
          purple: '#df69ba',
          pink: '#df69ba',
        },
      },
    ],
  },
  {
    id: 'kanagawa',
    name: 'Kanagawa',
    description: 'Ink and paper, after Hokusai’s Great Wave.',
    variants: [
      {
        id: 'kanagawa-wave',
        family: 'kanagawa',
        name: 'Wave',
        mode: 'dark',
        source: sources.kanagawa,
        canvas: '#16161d',
        surface: '#1f1f28',
        overlay: '#363646',
        ink: '#dcd7ba',
        muted: '#a6a69c',
        accent: '#7e9cd8',
        palette: {
          red: '#ff5d62',
          orange: '#ffa066',
          yellow: '#e6c384',
          green: '#98bb6c',
          cyan: '#7aa89f',
          blue: '#7e9cd8',
          purple: '#957fb8',
          pink: '#d27e99',
        },
      },
      {
        id: 'kanagawa-lotus',
        family: 'kanagawa',
        name: 'Lotus',
        mode: 'light',
        source: sources.kanagawa,
        canvas: '#e7dba0',
        surface: '#f2ecbc',
        overlay: '#dcd5ac',
        ink: '#545464',
        muted: '#636055', // WCAG: was #716e61 (3.66:1), lightness nudged to 4.51:1
        accent: '#4d699b',
        palette: {
          red: '#c84053',
          orange: '#cc6d00',
          yellow: '#836f4a',
          green: '#6f894e',
          cyan: '#597b75',
          blue: '#4d699b',
          purple: '#624c83',
          pink: '#b35b79',
        },
      },
    ],
  },
  {
    id: 'github',
    name: 'GitHub',
    description: 'Familiar, neutral and high contrast.',
    variants: [
      {
        id: 'github-dark',
        family: 'github',
        name: 'Dark',
        mode: 'dark',
        source: sources.github,
        canvas: '#010409',
        surface: '#0d1117',
        overlay: '#30363d',
        ink: '#e6edf3',
        muted: '#8d96a0',
        accent: '#4493f8',
        palette: {
          red: '#f85149',
          orange: '#db6d28',
          yellow: '#d29922',
          green: '#3fb950',
          cyan: '#39c5cf',
          blue: '#4493f8',
          purple: '#ab7df8',
          pink: '#db61a2',
        },
      },
      {
        id: 'github-light',
        family: 'github',
        name: 'Light',
        mode: 'light',
        source: sources.github,
        canvas: '#f6f8fa',
        surface: '#ffffff',
        overlay: '#d1d9e0',
        ink: '#1f2328',
        muted: '#59636e',
        accent: '#0969da',
        palette: {
          red: '#d1242f',
          orange: '#bc4c00',
          yellow: '#9a6700',
          green: '#1a7f37',
          cyan: '#1b7c83',
          blue: '#0969da',
          purple: '#8250df',
          pink: '#bf3989',
        },
      },
    ],
  },
]

export const schemeFamilies = families
export const schemes: Scheme[] = families.flatMap((family) => family.variants)
const byId = new Map(schemes.map((scheme) => [scheme.id, scheme]))

export const defaultSchemes = { dark: 'ircord-dark', light: 'ircord-light' } as const

export function findScheme(id: unknown): Scheme | undefined {
  return typeof id === 'string' ? byId.get(id) : undefined
}

/** A scheme for the requested mode: the given one if it matches, else its family's sibling, else IRCord. */
export function schemeFor(id: unknown, mode: SchemeMode): Scheme {
  const scheme = findScheme(id)
  if (scheme?.mode === mode) return scheme
  const sibling = schemes.find((item) => item.family === scheme?.family && item.mode === mode)
  return sibling || byId.get(defaultSchemes[mode])!
}

export const themeTokens = [
  'rail',
  'sidebar',
  'main',
  'panel',
  'raised',
  'hover',
  'input',
  'border',
  'control-border',
  'text',
  'body',
  'muted',
  'faint',
  'accent',
  'accent-ink',
  'selection',
  'mint',
  'danger',
  'yellow',
  'code-keyword',
  'code-string',
  'code-number',
  'code-comment',
  'code-function',
  'code-type',
  'code-property',
  'code-meta',
  'code-deletion',
] as const
export type ThemeToken = (typeof themeTokens)[number]

/** Syntax colours for fenced code, from the scheme's palette, readable on the code background. */
function codeTokens({ palette, muted }: Scheme, background: string) {
  const on = (color: string) => readableOn(color, background, 4.5)
  return {
    'code-keyword': on(palette.purple),
    'code-string': on(palette.green),
    'code-number': on(palette.orange),
    'code-comment': on(muted),
    'code-function': on(palette.blue),
    'code-type': on(palette.yellow),
    'code-property': on(palette.cyan),
    'code-meta': on(palette.pink),
    'code-deletion': on(palette.red),
  }
}

/**
 * Concrete colours for every interface token. `accentSource` replaces the scheme accent and is
 * adjusted until it reads clearly on the conversation background.
 */
export function schemeTokens(
  scheme: Scheme,
  accentSource?: string | null,
): Record<ThemeToken, string> {
  const { canvas, surface, overlay, ink, muted } = scheme
  const derived: Record<ThemeToken, string> = {
    rail: canvas,
    sidebar: mix(canvas, surface, 0.45),
    main: surface,
    panel: mix(canvas, surface, 0.7),
    raised: mix(surface, overlay, 0.4),
    hover: mix(surface, overlay, 0.6),
    input: mix(surface, overlay, 0.35),
    border: mix(surface, overlay, 0.75),
    'control-border': '',
    text: ink,
    body: mix(ink, muted, 0.3),
    muted,
    faint: mix(muted, surface, 0.18),
    accent: scheme.accent,
    'accent-ink': '',
    selection: '',
    // Status colours are fitted to text contrast below, with the other text tokens.
    mint: scheme.palette.green,
    danger: scheme.palette.red,
    yellow: scheme.palette.yellow,
    ...codeTokens(scheme, scheme.tokens?.rail ?? canvas),
    ...scheme.tokens,
  }
  if (isHex(accentSource)) derived.accent = accentSource
  // WCAG: text tokens (the accent colours links and badges) are nudged until they read at 4.5:1
  // on every surface text sits on, not only the conversation. The selected row is tinted with the
  // accent, so a brighter accent brightens its own background: fit both until they settle.
  const surfaces = [
    derived.rail,
    derived.sidebar,
    derived.main,
    derived.panel,
    derived.raised,
    derived.hover,
    derived.input,
  ]
  const tint = scheme.mode === 'dark' ? 0.16 : 0.13
  derived.accent = readableOnAll(derived.accent, surfaces)
  for (let pass = 0; pass < 8; pass++) {
    derived.selection = mix(derived.main, derived.accent, tint)
    const accent = readableOnAll(derived.accent, [...surfaces, derived.selection])
    if (accent === derived.accent) break
    derived.accent = accent
  }
  surfaces.push(derived.selection)
  for (const token of ['text', 'body', 'muted', 'faint', 'mint', 'danger', 'yellow'] as const)
    derived[token] = readableOnAll(derived[token], surfaces)
  // Nudging can carry faint past muted; at most they match, so the hierarchy never inverts.
  if (contrast(derived.faint, derived.main) > contrast(derived.muted, derived.main))
    derived.faint = derived.muted
  derived['accent-ink'] = inkFor(derived.accent)
  // WCAG: field and checkbox edges reach 3:1 against the surfaces they sit on; `border` stays the
  // quiet divider colour.
  derived['control-border'] = readableOnAll(
    derived.border,
    [derived.input, derived.main, derived.panel, derived.raised, derived.sidebar],
    3,
  )
  return derived
}
