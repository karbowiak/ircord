type ResolvedMedia = {
  sourceUrl: string
  playbackUrl: string | null
  previewUrl: string | null
  type: 'video' | 'image' | null
}

function decodeEscaped(input: string): string {
  return input.replace(/\\\//g, '/')
}

function extractMetaContent(html: string, key: string): string | null {
  const pattern = new RegExp(`<meta[^>]+(?:property|name)=["']${key}["'][^>]+content=["']([^"']+)["'][^>]*>`, 'i')
  const match = html.match(pattern)
  return match?.[1] ? decodeEscaped(match[1]) : null
}

function extractByRegex(html: string, regex: RegExp): string | null {
  const match = html.match(regex)
  return match?.[1] ? decodeEscaped(match[1]) : null
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
  const response = await fetch(url, {
    headers: {
      'user-agent': 'ircord-media-resolver/1.0',
      accept: 'text/html,*/*;q=0.8',
    },
  })

  if (!response.ok) {
    return { sourceUrl: url, playbackUrl: null, previewUrl: null, type: null }
  }

  const html = await response.text()

  const ogVideo = extractMetaContent(html, 'og:video:secure_url')
    || extractMetaContent(html, 'og:video')
    || extractByRegex(html, /"contentUrl"\s*:\s*"([^"]+)"/i)
    || extractByRegex(html, /"mp4"\s*:\s*"([^"]+)"/i)
  const ogImage = extractMetaContent(html, 'og:image:secure_url')
    || extractMetaContent(html, 'og:image')
    || extractByRegex(html, /"thumbnailUrl"\s*:\s*"([^"]+)"/i)
    || extractByRegex(html, /"gif"\s*:\s*"([^"]+)"/i)

  if (ogVideo && isVideoUrl(ogVideo)) {
    return {
      sourceUrl: url,
      playbackUrl: ogVideo,
      previewUrl: ogImage && isImageUrl(ogImage) ? ogImage : ogVideo,
      type: 'video',
    }
  }

  if (ogImage && isImageUrl(ogImage)) {
    return {
      sourceUrl: url,
      playbackUrl: ogImage,
      previewUrl: ogImage,
      type: 'image',
    }
  }

  return { sourceUrl: url, playbackUrl: null, previewUrl: null, type: null }
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
      } satisfies ResolvedMedia
    }
  }

  if (isTenor || isGiphyPage) {
    return await resolveFromHtml(raw)
  }

  return {
    sourceUrl: raw,
    playbackUrl: null,
    previewUrl: null,
    type: null,
  } satisfies ResolvedMedia
})
