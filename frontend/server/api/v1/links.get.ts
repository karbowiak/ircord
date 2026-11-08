import { listLinks } from '../../utils/demo-links'
const list = (value: unknown) =>
  (Array.isArray(value) ? value : value === undefined ? [] : [value]).map(String).slice(0, 50)
export default defineEventHandler(async (event) => {
  try {
    const q = getQuery(event)
    return listLinks(
      await demoSession(event),
      {
        conversations: list(q.conversation),
        network: typeof q.network === 'string' ? q.network : '',
        q: typeof q.q === 'string' ? q.q.trim().slice(0, 200) : '',
        host: typeof q.host === 'string' ? q.host.toLowerCase() : '',
        kinds: list(q.kind),
      },
      typeof q.cursor === 'string' ? q.cursor : '',
      Math.min(50, Math.max(1, Number(q.limit) || 30)),
    )
  } catch (error) {
    routeError(error)
  }
})
