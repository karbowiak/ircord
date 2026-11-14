import type { H3Event } from 'h3'
import type { Account } from '../../shared/types/chat'
import { createIRCordClient } from '../../shared/api/client'

type Kind = 'appearance' | 'library' | 'preferences'
const client = (event: H3Event) =>
  createIRCordClient(goApiBase(event), {
    headers: { cookie: getHeader(event, 'cookie') ?? '' },
    fetch: (request) =>
      fetch(request, { signal: AbortSignal.any([request.signal, AbortSignal.timeout(5000)]) }),
  })

/**
 * The address the request came from: from the production server entry (server/entry.bun.ts),
 * or from the socket when the server has one (development).
 */
export function peerAddress(event: H3Event) {
  const context = (event.node.req as { __unenv__?: { peerAddress?: unknown } }).__unenv__
  if (typeof context?.peerAddress === 'string' && context.peerAddress) return context.peerAddress
  return event.node.req.socket?.remoteAddress || undefined
}

/**
 * X-Forwarded-For for the backend: the inbound chain with this request's peer appended. The
 * backend reads it from the right and only believes entries added by its trusted proxies
 * (IRCORD_TRUSTED_PROXIES), so a client can't choose its own address. Without a known peer
 * it is empty, so the backend counts the frontend rather than a client-supplied entry.
 */
export function forwardedFor(event: H3Event) {
  const peer = peerAddress(event)
  if (!peer) return ''
  const inbound = getRequestHeaders(event)['x-forwarded-for']?.trim()
  return [inbound, peer].filter(Boolean).join(', ')
}

/** Resolve once per request; no account's cookie is retained outside that request. */
export function goApiBase(event: H3Event) {
  const config = useRuntimeConfig(event)
  if (config.demo) return ''
  const explicit = config.backendUrl as string
  if (explicit) return `${explicit.replace(/\/$/, '')}/api/v1`
  const base = config.public.apiBase
  return /^https?:\/\//.test(base) ? base.replace(/\/$/, '') : ''
}
export async function backendAccount(event: H3Event): Promise<Account> {
  const result = await client(event).session()
  if (!result.account) throw createError({ statusCode: 401, statusMessage: 'Sign in to continue.' })
  return result.account
}
export const backendDocument = (event: H3Event, kind: Kind) => client(event).document(kind)
/** Compare-and-swap retries keep two devices or Nitro replicas from losing library edits. */
export async function changeBackendDocument<T>(
  event: H3Event,
  kind: Kind,
  change: (data: unknown) => T,
) {
  for (let attempt = 0; attempt < 5; attempt++) {
    const saved = await backendDocument(event, kind)
    try {
      const data = change(saved.data)
      await client(event).saveDocument(kind, { data, revision: saved.revision })
      return data
    } catch (error) {
      if ((error as { status?: number }).status !== 409 || attempt === 4) throw error
    }
  }
  throw createError({
    statusCode: 409,
    statusMessage: 'Saved data changed on another device. Try again.',
  })
}
