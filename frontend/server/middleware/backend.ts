import { forwardedFor, goApiBase } from '../utils/backend'

/** Keep live HTTP, cookies and shareable files on the frontend origin. */
export default defineEventHandler(async (event) => {
  const path = event.path.split('?')[0]!
  if (
    (path.startsWith('/api/app/') || path.startsWith('/api/v1/')) &&
    !['GET', 'HEAD', 'OPTIONS'].includes(event.method)
  ) {
    const origin = getHeader(event, 'origin')
    const fetchSite = getHeader(event, 'sec-fetch-site')
    const configured = useRuntimeConfig(event).siteUrl as string
    let expected: string
    try {
      // Do not trust forwarded host or protocol from the request. Deployments behind TLS
      // termination must configure their public origin.
      const url = configured
        ? new URL(configured)
        : getRequestURL(event, { xForwardedHost: false, xForwardedProto: false })
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password)
        throw new Error('Invalid public origin')
      expected = url.origin
    } catch {
      throw createError({ statusCode: 403, statusMessage: 'Request origin rejected.' })
    }
    // Browsers cannot set these headers. Keep headerless non-browser clients working, but
    // require an exact origin whenever a browser supplies Fetch Metadata.
    if (
      (origin !== undefined && origin !== expected) ||
      (fetchSite !== undefined && (fetchSite !== 'same-origin' || !origin))
    )
      throw createError({ statusCode: 403, statusMessage: 'Request origin rejected.' })
  }
  if (
    !event.path.startsWith('/api/v1/') &&
    !event.path.startsWith('/files/') &&
    !event.path.startsWith('/emotes/') &&
    !event.path.startsWith('/schemas/') &&
    !['/docs', '/openapi.json', '/openapi.yaml'].includes(event.path.split('?')[0]!)
  )
    return
  const backend = goApiBase(event).replace(/\/api\/v1$/, '')
  if (!backend) return
  try {
    return await proxyRequest(event, backend + event.path, {
      streamRequest: true,
      headers: {
        'x-forwarded-for': forwardedFor(event),
        'x-forwarded-host': getHeader(event, 'host') ?? '',
      },
    })
  } catch {
    throw createError({ statusCode: 503, statusMessage: 'The IRCord backend is unavailable.' })
  }
})
