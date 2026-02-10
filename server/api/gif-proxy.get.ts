export default defineEventHandler(async (event) => {
  const query = getQuery(event)
  const rawUrl = query.url

  if (typeof rawUrl !== 'string' || rawUrl.trim().length === 0) {
    throw createError({ statusCode: 400, statusMessage: 'Missing url query parameter' })
  }

  let parsed: URL
  try {
    parsed = new URL(rawUrl)
  } catch {
    throw createError({ statusCode: 400, statusMessage: 'Invalid url query parameter' })
  }

  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw createError({ statusCode: 400, statusMessage: 'Only http/https URLs are supported' })
  }

  const upstream = await fetch(parsed.toString(), {
    headers: {
      'user-agent': 'ircord-gif-proxy/1.0',
      accept: '*/*',
    },
  })

  if (!upstream.ok) {
    throw createError({
      statusCode: upstream.status,
      statusMessage: `Failed to fetch media (${upstream.status})`,
    })
  }

  const contentType = upstream.headers.get('content-type') || 'application/octet-stream'
  const data = await upstream.arrayBuffer()

  return new Response(data, {
    headers: {
      'content-type': contentType,
      'cache-control': 'public, max-age=3600',
    },
  })
})
