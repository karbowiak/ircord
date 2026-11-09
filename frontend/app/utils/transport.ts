/** The UI uses a stable self sentinel; transport identity always remains the authenticated account. */
import validateEvent from '../../shared/generated/event-validator.js'
import type { ServerEvent } from '../../shared/types/chat'
const record = (value: unknown): value is Record<string, unknown> => !!value && typeof value === 'object' && !Array.isArray(value)
let selfId = ''
/** Translate UI member IDs back to the authenticated transport identity. */
export const transportMemberId = (id: string) => id === 'you' && selfId ? selfId : id
export function normalizeTransport<T>(value: T): T {
  if (record(value) && record(value.user) && typeof value.user.id === 'string') selfId = value.user.id === 'you' ? '' : value.user.id
  if (!selfId) return value
  const memberId = (id: unknown) => (id === selfId ? 'you' : id)
  const visit = (item: unknown): unknown => {
    if (Array.isArray(item)) return item.map(visit)
    if (!record(item)) return item
    const result: Record<string, unknown> = {}
    for (const [key, field] of Object.entries(item)) {
      if (['senderId', 'actorId', 'targetId', 'memberId'].includes(key)) result[key] = memberId(field)
      else if (key === 'memberIds' && Array.isArray(field)) result[key] = field.map(memberId)
      else if (key === 'memberModes' && field && typeof field === 'object')
        result[key] = Object.fromEntries(
          Object.entries(field).map(([id, modes]) => [memberId(id), modes]),
        )
      else if (key === 'id' && typeof item.nick === 'string' && typeof item.displayName === 'string') result[key] = memberId(field)
      else result[key] = visit(field)
    }
    return result
  }
  return visit(value) as T
}
/** Generated validator is compiled at build time and works under the app's CSP. */
export function parseServerEvent(raw: string): ServerEvent {
  const value: unknown = JSON.parse(raw)
  if (!validateEvent(value)) throw new Error('The server sent an invalid event.')
  return normalizeTransport(value)
}
export function realtimeUrl(apiBase: string, explicit: string, location: Pick<Location, 'href'>) {
  if (explicit) return explicit
  const url = new URL(`${apiBase.replace(/\/$/, '')}/events`, location.href)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}
