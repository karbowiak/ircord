type ResolvedMedia = {
  sourceUrl: string
  playbackUrl: string | null
  previewUrl: string | null
  type: 'video' | 'image' | null
  title: string | null
  description: string | null
  siteName: string | null
  canonicalUrl: string | null
}

function decodeEscaped(input: string): string {
  return input.replace(/\\\//g, '/')
}

function decodeHtmlEntities(input: string): string {
  return input
    .replace(/&amp;/gi, '&')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;|&apos;/gi, "'")
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
}

function normalizeMetaValue(value: string | null): string | null {
  if (!value) return null
  const trimmed = decodeHtmlEntities(decodeEscaped(value)).trim()
  return trimmed.length ? trimmed : null
}

function escapeRegex(input: string): string {
  return input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function getAttributeValue(tag: string, attribute: string): string | null {
  const escaped = escapeRegex(attribute)
  const attrRegex = new RegExp(`${escaped}\\s*=\\s*(?:"([^"]*)"|'([^']*)'|([^\\s>]+))`, 'i')
  const match = tag.match(attrRegex)
  const value = match?.[1] ?? match?.[2] ?? match?.[3] ?? null
  return normalizeMetaValue(value)
}

function isBlockedHost(hostname: string): boolean {
  const host = hostname.toLowerCase()
  if (host === 'localhost' || host === '::1') return true
  if (host === '0.0.0.0') return true
  if (host.startsWith('127.')) return true
  if (host.startsWith('10.')) return true
  if (host.startsWith('192.168.')) return true
  if (host.startsWith('169.254.')) return true
  if (/^172\.(1[6-9]|2\d|3[0-1])\./.test(host)) return true
  return false
}

function extractMetaContent(html: string, key: string): string | null {
  const metaTags = html.match(/<meta\b[^>]*>/gi) || []
  const lowerKey = key.toLowerCase()

  for (const tag of metaTags) {
    const name = (getAttributeValue(tag, 'property') || getAttributeValue(tag, 'name') || '').toLowerCase()
    if (name !== lowerKey) continue
    const content = getAttributeValue(tag, 'content')
    if (content) return content
  }

  return null
}

function extractByRegex(html: string, regex: RegExp): string | null {
  const match = html.match(regex)
  return normalizeMetaValue(match?.[1] || null)
}

function asAbsoluteUrl(baseUrl: string, maybeRelative: string | null): string | null {
  if (!maybeRelative) return null
  try {
    return new URL(maybeRelative, baseUrl).toString()
  } catch {
    return maybeRelative
  }
}

function isHttpUrl(url: string): boolean {
  try {
    const parsed = new URL(url)
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

function extractCanonicalUrl(html: string, baseUrl: string): string | null {
  const linkTags = html.match(/<link\b[^>]*>/gi) || []

  for (const tag of linkTags) {
    const rel = (getAttributeValue(tag, 'rel') || '').toLowerCase()
    if (!rel.split(/\s+/).includes('canonical')) continue
    const href = getAttributeValue(tag, 'href')
    const absolute = asAbsoluteUrl(baseUrl, href)
    if (absolute && isHttpUrl(absolute)) return absolute
  }

  return null
}

function extractFirstJsonLdObject(html: string): Record<string, unknown> | null {
  const scriptMatches = html.match(/<script\b[^>]*type\s*=\s*(?:"application\/ld\+json"|'application\/ld\+json'|application\/ld\+json)[^>]*>[\s\S]*?<\/script>/gi) || []

  for (const scriptTag of scriptMatches) {
    const jsonText = scriptTag
      .replace(/^<script[^>]*>/i, '')
      .replace(/<\/script>$/i, '')
      .trim()

    if (!jsonText) continue

    try {
      const parsed = JSON.parse(jsonText) as unknown
      if (Array.isArray(parsed)) {
        const firstObject = parsed.find(entry => entry && typeof entry === 'object') as Record<string, unknown> | undefined
        if (firstObject) return firstObject
      }

      if (parsed && typeof parsed === 'object') {
        return parsed as Record<string, unknown>
      }
    } catch {
      continue
    }
  }

  return null
}

function extractOembedUrl(html: string, baseUrl: string): string | null {
  const linkTags = html.match(/<link\b[^>]*>/gi) || []

  for (const tag of linkTags) {
    const rel = (getAttributeValue(tag, 'rel') || '').toLowerCase()
    const type = (getAttributeValue(tag, 'type') || '').toLowerCase()
    const href = getAttributeValue(tag, 'href')

    if (!rel.includes('alternate')) continue
    if (type !== 'application/json+oembed' && type !== 'text/xml+oembed') continue

    const absolute = asAbsoluteUrl(baseUrl, href)
    if (absolute && isHttpUrl(absolute)) return absolute
  }

  return null
}

type OembedData = {
  title: string | null
  description: string | null
  siteName: string | null
  previewUrl: string | null
  playbackUrl: string | null
  type: 'video' | 'image' | null
}

async function fetchOembedData(oembedUrl: string): Promise<OembedData | null> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 4500)

  try {
    const response = await fetch(oembedUrl, {
      headers: {
        'user-agent': 'ircord-media-resolver/1.0',
        accept: 'application/json,text/json,*/*;q=0.8',
      },
      signal: controller.signal,
    })

    clearTimeout(timeout)

    if (!response.ok) return null
    const contentType = response.headers.get('content-type') || ''
    if (!contentType.includes('json')) return null

    const payload = await response.json() as Record<string, unknown>
    const type = typeof payload.type === 'string' ? payload.type.toLowerCase() : ''

    const title = normalizeMetaValue(typeof payload.title === 'string' ? payload.title : null)
    const description = normalizeMetaValue(typeof payload.description === 'string' ? payload.description : null)
    const siteName = normalizeMetaValue(
      typeof payload.provider_name === 'string'
        ? payload.provider_name
        : (typeof payload.author_name === 'string' ? payload.author_name : null)
    )

    const previewUrl = normalizeMetaValue(
      typeof payload.thumbnail_url === 'string'
        ? payload.thumbnail_url
        : (typeof payload.url === 'string' ? payload.url : null)
    )

    const html = normalizeMetaValue(typeof payload.html === 'string' ? payload.html : null)
    const iframeSrc = html
      ? extractByRegex(html, /<iframe[^>]+src=["']([^"']+)["']/i)
      : null

    const playbackCandidate = normalizeMetaValue(
      typeof payload.url === 'string'
        ? payload.url
        : iframeSrc
    )

    const playbackUrl = playbackCandidate && isHttpUrl(playbackCandidate)
      ? playbackCandidate
      : null

    const resolvedType: 'video' | 'image' | null =
      type === 'video' ? 'video'
      : type === 'photo' ? 'image'
      : (playbackUrl && isVideoUrl(playbackUrl) ? 'video' : null)

    return {
      title,
      description,
      siteName,
      previewUrl: previewUrl && isHttpUrl(previewUrl) ? previewUrl : null,
      playbackUrl: playbackUrl && isVideoUrl(playbackUrl) ? playbackUrl : null,
      type: resolvedType,
    }
  } catch {
    clearTimeout(timeout)
    return null
  }
}

function getGiphyIdFromUrl(url: URL): string | null {
  const path = url.pathname.replace(/\/$/, '')
  const lastSegment = path.split('/').filter(Boolean).pop()
  if (!lastSegment) return null
  const idMatch = lastSegment.match(/-([a-zA-Z0-9]+)$/)
  return idMatch?.[1] ?? null
}

function isVideoUrl(url: string): boolean {
  return /\.(mp4|webm)(?:\?.*)?$/i.test(url)
}

function isImageUrl(url: string): boolean {
  return /\.(gif|webp|png|jpe?g|avif)(?:\?.*)?$/i.test(url)
}

async function resolveFromHtml(url: string): Promise<ResolvedMedia> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), 5000)

  const response = await fetch(url, {
    headers: {
      'user-agent': 'ircord-media-resolver/1.0',
      accept: 'text/html,*/*;q=0.8',
    },
    signal: controller.signal,
  })

  clearTimeout(timeout)

  if (!response.ok) {
    return { sourceUrl: url, playbackUrl: null, previewUrl: null, type: null, title: null, description: null, siteName: null, canonicalUrl: null }
  }

  const contentType = response.headers.get('content-type') || ''
  if (!contentType.includes('text/html')) {
    return { sourceUrl: url, playbackUrl: null, previewUrl: null, type: null, title: null, description: null, siteName: null, canonicalUrl: null }
  }

  const html = await response.text()
  const oembedUrl = extractOembedUrl(html, url)
  const oembed = oembedUrl ? await fetchOembedData(oembedUrl) : null
  const jsonLd = extractFirstJsonLdObject(html)

  const title = oembed?.title
    || extractMetaContent(html, 'og:title')
    || extractMetaContent(html, 'twitter:title')
    || normalizeMetaValue(typeof jsonLd?.headline === 'string' ? jsonLd.headline : null)
    || extractByRegex(html, /<title[^>]*>([^<]+)<\/title>/i)
  const description = oembed?.description
    || extractMetaContent(html, 'og:description')
    || extractMetaContent(html, 'twitter:description')
    || extractMetaContent(html, 'description')
    || normalizeMetaValue(typeof jsonLd?.description === 'string' ? jsonLd.description : null)
  const siteName = oembed?.siteName || extractMetaContent(html, 'og:site_name') || extractMetaContent(html, 'twitter:site')
  const canonicalUrl = extractCanonicalUrl(html, url) || asAbsoluteUrl(url, extractMetaContent(html, 'og:url'))

  const ogType = (extractMetaContent(html, 'og:type') || '').toLowerCase()
  const twitterCard = (extractMetaContent(html, 'twitter:card') || '').toLowerCase()

  const ogVideo = oembed?.playbackUrl || asAbsoluteUrl(url,
    extractMetaContent(html, 'og:video:secure_url')
    || extractMetaContent(html, 'og:video')
    || extractMetaContent(html, 'twitter:player:stream')
    || normalizeMetaValue(typeof jsonLd?.contentUrl === 'string' ? jsonLd.contentUrl : null)
    || extractByRegex(html, /"contentUrl"\s*:\s*"([^"]+)"/i)
    || extractByRegex(html, /"mp4"\s*:\s*"([^"]+)"/i)
  )
  const ogImage = oembed?.previewUrl || asAbsoluteUrl(url,
    extractMetaContent(html, 'og:image:secure_url')
    || extractMetaContent(html, 'og:image')
    || extractMetaContent(html, 'twitter:image')
    || normalizeMetaValue(typeof jsonLd?.thumbnailUrl === 'string' ? jsonLd.thumbnailUrl : null)
    || normalizeMetaValue(typeof jsonLd?.image === 'string' ? jsonLd.image : null)
    || extractByRegex(html, /"thumbnailUrl"\s*:\s*"([^"]+)"/i)
    || extractByRegex(html, /"gif"\s*:\s*"([^"]+)"/i)
  )

  if (ogVideo && (isVideoUrl(ogVideo) || ogType.startsWith('video') || twitterCard === 'player') && isHttpUrl(ogVideo)) {
    return {
      sourceUrl: url,
      playbackUrl: ogVideo,
      previewUrl: ogImage && isImageUrl(ogImage) ? ogImage : ogVideo,
      type: 'video',
      title,
      description,
      siteName,
      canonicalUrl,
    }
  }

  if (ogImage && (isImageUrl(ogImage) || isHttpUrl(ogImage))) {
    return {
      sourceUrl: url,
      playbackUrl: ogImage,
      previewUrl: ogImage,
      type: 'image',
      title,
      description,
      siteName,
      canonicalUrl,
    }
  }

  return { sourceUrl: url, playbackUrl: null, previewUrl: null, type: null, title, description, siteName, canonicalUrl }
}

export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const raw = typeof query.url === 'string' ? query.url : ''

  if (!raw) {
    throw createError({ statusCode: 400, statusMessage: 'Missing url parameter' })
  }

  let parsed: URL
  try {
    parsed = new URL(raw)
  } catch {
    throw createError({ statusCode: 400, statusMessage: 'Invalid URL' })
  }

  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw createError({ statusCode: 400, statusMessage: 'Unsupported URL protocol' })
  }

  if (isBlockedHost(parsed.hostname)) {
    throw createError({ statusCode: 400, statusMessage: 'Blocked hostname' })
  }

  const hostname = parsed.hostname.toLowerCase()
  const isTenor = hostname.includes('tenor.com') && parsed.pathname.startsWith('/view/')
  const isGiphyPage = hostname.includes('giphy.com') && (parsed.pathname.startsWith('/gifs/') || parsed.pathname.startsWith('/stickers/') || parsed.pathname.startsWith('/clips/'))

  if (isGiphyPage) {
    const id = getGiphyIdFromUrl(parsed)
    if (id) {
      return {
        sourceUrl: raw,
        playbackUrl: `https://media.giphy.com/media/${id}/giphy.mp4`,
        previewUrl: `https://media.giphy.com/media/${id}/giphy.webp`,
        type: 'video',
        title: null,
        description: null,
        siteName: 'GIPHY',
        canonicalUrl: raw,
      } satisfies ResolvedMedia
    }
  }

  if (isTenor || isGiphyPage) return await resolveFromHtml(raw)

  const generic = await resolveFromHtml(raw)

  if (generic.type === 'image' && generic.playbackUrl && generic.previewUrl === generic.playbackUrl && (generic.title || generic.description)) {
    generic.previewUrl = generic.playbackUrl
    generic.playbackUrl = null
    generic.type = null
  }

  if (generic.title || generic.description || generic.previewUrl || generic.playbackUrl) {
    return generic
  }

  return {
    sourceUrl: raw,
    playbackUrl: null,
    previewUrl: null,
    type: null,
    title: null,
    description: null,
    siteName: null,
    canonicalUrl: null,
  } satisfies ResolvedMedia
})
