import { listLinks } from '../../utils/demo-links'
/** An optional time parameter as ms since the epoch (0 when absent); invalid ones are refused. */
function instant(value: unknown) {
  if (value === undefined || value === '') return 0
  const at = typeof value === 'string' ? Date.parse(value) : NaN
  if (Number.isNaN(at)) throw createError({ statusCode: 422, statusMessage: 'invalid time' })
  return at
}
const list = (value: unknown) =>
  (Array.isArray(value) ? value : value === undefined ? [] : [value]).map(String).slice(0, 50)
export default defineEventHandler(async (event) => {
  try {
    const q = getQuery(event)
    const since = instant(q.since)
    const until = instant(q.until)
    if (since && until && until <= since)
      throw createError({ statusCode: 422, statusMessage: 'until must be after since' })
    return listLinks(
      await demoSession(event),
      {
        conversations: list(q.conversation),
        network: typeof q.network === 'string' ? q.network : '',
        q: typeof q.q === 'string' ? q.q.trim().slice(0, 200) : '',
        host: typeof q.host === 'string' ? q.host.toLowerCase() : '',
        kinds: list(q.kind),
        since,
        until,
      },
      typeof q.cursor === 'string' ? q.cursor : '',
      Math.min(50, Math.max(1, Number(q.limit) || 30)),
    )
  } catch (error) {
    routeError(error)
  }
})
