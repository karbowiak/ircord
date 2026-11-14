import satori from 'satori'
import interRegularUrl from '@fontsource/inter/files/inter-latin-400-normal.woff?url'
import interBoldUrl from '@fontsource/inter/files/inter-latin-700-normal.woff?url'

export interface SatoriLinkInput {
  url: string
  site?: string
  title?: string
  description?: string
}

type SatoriElement = Parameters<typeof satori>[0]
type SatoriFont = Parameters<typeof satori>[1]['fonts'][number]

interface PosterPalette {
  glowA: string
  glowB: string
  dot: string
  barStart: string
  barEnd: string
}

const POSTER_PALETTES: readonly PosterPalette[] = [
  {
    glowA: 'rgba(224, 108, 83, 0.34)',
    glowB: 'rgba(245, 158, 11, 0.20)',
    dot: '#e06c53',
    barStart: '#f43f5e',
    barEnd: '#f59e0b',
  },
  {
    glowA: 'rgba(244, 63, 94, 0.32)',
    glowB: 'rgba(251, 146, 60, 0.22)',
    dot: '#f43f5e',
    barStart: '#fb7185',
    barEnd: '#fb923c',
  },
  {
    glowA: 'rgba(234, 88, 12, 0.32)',
    glowB: 'rgba(244, 63, 94, 0.22)',
    dot: '#fb923c',
    barStart: '#f97316',
    barEnd: '#f43f5e',
  },
  {
    glowA: 'rgba(217, 70, 239, 0.24)',
    glowB: 'rgba(244, 63, 94, 0.24)',
    dot: '#f472b6',
    barStart: '#ec4899',
    barEnd: '#f59e0b',
  },
  {
    glowA: 'rgba(245, 158, 11, 0.28)',
    glowB: 'rgba(224, 108, 83, 0.24)',
    dot: '#fbbf24',
    barStart: '#f59e0b',
    barEnd: '#e06c53',
  },
] as const

const MAX_CACHE_ENTRIES = 200
const svgCache = new Map<string, string>()
let fontsPromise: Promise<SatoriFont[]> | undefined

function hashString(value: string) {
  let hash = 2166136261
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

function pickPalette(seed: string): PosterPalette {
  const index = hashString(seed) % POSTER_PALETTES.length
  return POSTER_PALETTES[index]!
}

function cleanText(value: string | undefined, maxLength: number) {
  if (!value) return ''
  // Normalize whitespace and trim cleanly; Satori handles Unicode via our loaded Latin WOFFs.
  const collapsed = value.replace(/\s+/g, ' ').trim()
  if (collapsed.length <= maxLength) return collapsed
  return `${collapsed.slice(0, maxLength - 1).trimEnd()}…`
}

function extractHostAndPath(rawUrl: string, site?: string) {
  try {
    const parsed = new URL(rawUrl)
    const host = site?.trim() || parsed.hostname.replace(/^www\./, '')
    const path = parsed.pathname === '/' ? '' : parsed.pathname
    return { host, path: cleanText(path, 52) }
  } catch {
    return { host: site?.trim() || 'link', path: '' }
  }
}

function svgToDataUrl(svg: string) {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg).replaceAll("'", '%27')}`
}

async function loadFonts(): Promise<SatoriFont[]> {
  if (!fontsPromise) {
    fontsPromise = Promise.all([
      fetch(interRegularUrl).then((response) => {
        if (!response.ok) throw new Error(`Font fetch failed: ${response.status}`)
        return response.arrayBuffer()
      }),
      fetch(interBoldUrl).then((response) => {
        if (!response.ok) throw new Error(`Font fetch failed: ${response.status}`)
        return response.arrayBuffer()
      }),
    ])
      .then(([regular, bold]): SatoriFont[] => [
        { name: 'Inter', data: regular, weight: 400, style: 'normal' },
        { name: 'Inter', data: bold, weight: 700, style: 'normal' },
      ])
      .catch((error) => {
        fontsPromise = undefined
        throw error
      })
  }
  return fontsPromise
}

/** Synchronous Hibiki-palette SVG backdrop returned immediately before Satori finishes rendering. */
export function fallbackLinkSvgDataUrl(input: SatoriLinkInput): string {
  const { host } = extractHostAndPath(input.url, input.site)
  const palette = pickPalette(host || input.url)
  const svg = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360" viewBox="0 0 640 360">',
    '<defs>',
    '<radialGradient id="g1" cx="14%" cy="8%" r="72%">',
    `<stop offset="0%" stop-color="${palette.glowA}"/>`,
    '<stop offset="100%" stop-color="rgba(20,17,17,0)"/>',
    '</radialGradient>',
    '<radialGradient id="g2" cx="84%" cy="82%" r="64%">',
    `<stop offset="0%" stop-color="${palette.glowB}"/>`,
    '<stop offset="100%" stop-color="rgba(20,17,17,0)"/>',
    '</radialGradient>',
    `<linearGradient id="bar" x1="0%" y1="0%" x2="100%" y2="0%"><stop offset="0%" stop-color="${palette.barStart}"/><stop offset="100%" stop-color="${palette.barEnd}"/></linearGradient>`,
    '</defs>',
    '<rect width="640" height="360" fill="#141111"/>',
    '<rect width="640" height="360" fill="url(#g1)"/>',
    '<rect width="640" height="360" fill="url(#g2)"/>',
    '<circle cx="548" cy="112" r="96" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="1"/>',
    '<circle cx="548" cy="112" r="54" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="1"/>',
    '<rect x="36" y="314" width="96" height="4" rx="2" fill="url(#bar)"/>',
    '</svg>',
  ].join('')
  return svgToDataUrl(svg)
}

/** Generates a 16:9 Hibiki-styled SVG poster via Satori for links without preview images. */
export async function renderSatoriLinkSvgDataUrl(input: SatoriLinkInput): Promise<string> {
  const { host, path } = extractHostAndPath(input.url, input.site)
  const title = cleanText(input.title, 92) || host
  const description = cleanText(input.description, 120) || (path ? `${host}${path}` : input.url)
  const cacheKey = `${input.url}\n${host}\n${title}\n${description}`
  const cached = svgCache.get(cacheKey)
  if (cached) return cached

  const palette = pickPalette(host || input.url)

  try {
    const fonts = await loadFonts()
    const tree = {
      type: 'div',
      props: {
        style: {
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          width: '640px',
          height: '360px',
          padding: '32px 36px 28px',
          backgroundColor: '#141111',
          backgroundImage: `radial-gradient(circle at 14% 8%, ${palette.glowA} 0%, transparent 56%), radial-gradient(circle at 86% 84%, ${palette.glowB} 0%, transparent 54%)`,
          color: '#f5eff2',
          fontFamily: 'Inter',
          position: 'relative',
          overflow: 'hidden',
        },
        children: [
          {
            type: 'div',
            props: {
              style: {
                position: 'absolute',
                right: '-44px',
                top: '-44px',
                width: '210px',
                height: '210px',
                borderRadius: '9999px',
                border: '1px solid rgba(255, 255, 255, 0.08)',
                display: 'flex',
              },
            },
          },
          {
            type: 'div',
            props: {
              style: {
                position: 'absolute',
                right: '14px',
                top: '14px',
                width: '94px',
                height: '94px',
                borderRadius: '9999px',
                border: '1px solid rgba(255, 255, 255, 0.11)',
                display: 'flex',
              },
            },
          },
          {
            type: 'div',
            props: {
              style: {
                display: 'flex',
                alignItems: 'center',
              },
              children: [
                {
                  type: 'div',
                  props: {
                    style: {
                      display: 'flex',
                      alignItems: 'center',
                      gap: '8px',
                      padding: '6px 14px',
                      borderRadius: '9999px',
                      backgroundColor: 'rgba(255, 255, 255, 0.08)',
                      border: '1px solid rgba(255, 255, 255, 0.14)',
                      fontSize: '14px',
                      fontWeight: 700,
                      color: '#f5eff2',
                    },
                    children: [
                      {
                        type: 'div',
                        props: {
                          style: {
                            width: '8px',
                            height: '8px',
                            borderRadius: '9999px',
                            backgroundColor: palette.dot,
                            display: 'flex',
                          },
                        },
                      },
                      host,
                    ],
                  },
                },
              ],
            },
          },
          {
            type: 'div',
            props: {
              style: {
                display: 'flex',
                flexDirection: 'column',
                gap: '10px',
                marginTop: '12px',
                marginBottom: '12px',
              },
              children: [
                {
                  type: 'div',
                  props: {
                    style: {
                      fontSize: title.length > 56 ? '28px' : '33px',
                      fontWeight: 700,
                      lineHeight: 1.2,
                      letterSpacing: '-0.02em',
                      color: '#f7f2f4',
                      display: 'flex',
                    },
                    children: title,
                  },
                },
                {
                  type: 'div',
                  props: {
                    style: {
                      fontSize: '16px',
                      fontWeight: 400,
                      lineHeight: 1.4,
                      color: '#b4a8af',
                      display: 'flex',
                    },
                    children: description,
                  },
                },
              ],
            },
          },
          {
            type: 'div',
            props: {
              style: {
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
              },
              children: [
                {
                  type: 'div',
                  props: {
                    style: {
                      width: '84px',
                      height: '4px',
                      borderRadius: '9999px',
                      backgroundImage: `linear-gradient(90deg, ${palette.barStart}, ${palette.barEnd})`,
                      display: 'flex',
                    },
                  },
                },
              ],
            },
          },
        ],
      },
    } as unknown as SatoriElement

    const svg = await satori(tree, { width: 640, height: 360, fonts })
    const dataUrl = svgToDataUrl(svg)
    if (svgCache.size >= MAX_CACHE_ENTRIES) {
      const oldest = svgCache.keys().next().value
      if (oldest) svgCache.delete(oldest)
    }
    svgCache.set(cacheKey, dataUrl)
    return dataUrl
  } catch {
    return fallbackLinkSvgDataUrl(input)
  }
}
