import { goApiBase } from '../utils/backend'

/** Keep live HTTP, cookies and shareable files on the frontend origin. */
export default defineEventHandler(async event => {
  if (!event.path.startsWith('/api/v1/') && !event.path.startsWith('/files/') && !event.path.startsWith('/schemas/') && !['/docs', '/openapi.json', '/openapi.yaml'].includes(event.path.split('?')[0]!)) return
  const backend = goApiBase(event).replace(/\/api\/v1$/, '')
  if (!backend) return
  try {
    return await proxyRequest(event, backend + event.path, {
      streamRequest: true,
      headers: { 'x-forwarded-for': getRequestIP(event) ?? '', 'x-forwarded-host': getHeader(event, 'host') ?? '' },
    })
  } catch {
    throw createError({ statusCode: 503, statusMessage: 'The IRCord backend is unavailable.' })
  }
})
