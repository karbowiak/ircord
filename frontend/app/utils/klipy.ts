import type { GifCategory, GifItem, GifPage } from '../../shared/types/media'
import { normalizeKlipyPage } from '../../shared/utils/media'

// Klipy's integration model is direct browser requests with the instance's public app key.
async function klipy(
  appKey: string,
  path: string,
  options: {
    query?: Record<string, string | number>
    method?: string
    body?: unknown
    signal?: AbortSignal
  } = {},
) {
  if (!appKey) throw new Error('GIF search is not configured on this instance.')
  const url = new URL(`https://api.klipy.com/api/v1/${appKey}/gifs/${path}`)
  for (const [key, value] of Object.entries(options.query ?? {}))
    url.searchParams.set(key, String(value))
  let response: Response
  try {
    const timeout = AbortSignal.timeout(12_000)
    response = await fetch(url, {
      method: options.method ?? 'GET',
      signal: options.signal ? AbortSignal.any([options.signal, timeout]) : timeout,
      ...(options.body
        ? { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(options.body) }
        : {}),
    })
  } catch {
    if (options.signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
    throw new Error('Couldn’t reach Klipy. Check your connection and try again.')
  }
  if (!response.ok)
    throw new Error(
      response.status === 429
        ? 'GIF search is busy. Try again in a moment.'
        : response.status === 401 || response.status === 403
          ? 'Klipy didn’t accept this instance’s app key.'
          : 'Klipy couldn’t load these GIFs. Please try again.',
    )
  try {
    return await response.json()
  } catch {
    throw new Error('Klipy returned an unreadable response.')
  }
}

const language = () => navigator.language.split('-')[0] || 'en'

export async function klipyGifs(
  appKey: string,
  query: string,
  page: number,
  signal?: AbortSignal,
): Promise<GifPage> {
  const q = query.trim().slice(0, 100)
  return normalizeKlipyPage(
    await klipy(appKey, q ? 'search' : 'trending', {
      query: {
        per_page: 24,
        page,
        locale: language(),
        content_filter: 'high',
        format_filter: 'gif,webp,jpg',
        ...(q ? { q } : {}),
      },
      signal,
    }),
  )
}

export async function klipyCategories(
  appKey: string,
  signal?: AbortSignal,
): Promise<GifCategory[]> {
  const response = await klipy(appKey, 'categories', {
    query: { locale: navigator.language.replace('-', '_') },
    signal,
  })
  const rows = response?.data?.categories
  if (!Array.isArray(rows)) return []
  return rows.slice(0, 16).map((row: Record<string, unknown>) => ({
    name: String(row.category),
    query: String(row.query),
    previewUrl: String(row.preview_url),
  }))
}

/** Full items for slugs; used once to migrate favourites saved before they kept their URLs. */
export async function klipyItems(appKey: string, slugs: string[]): Promise<GifItem[]> {
  if (!slugs.length) return []
  return normalizeKlipyPage(await klipy(appKey, 'items', { query: { slugs: slugs.join(',') } }))
    .items
}

/** Registers a share with Klipy, as its terms ask. Failures never block sending. */
export function klipyShare(appKey: string, item: GifItem, query = '') {
  void klipy(appKey, `share/${encodeURIComponent(item.slug)}`, {
    method: 'POST',
    body: query ? { q: query } : {},
  }).catch(() => {})
}
