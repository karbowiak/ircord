import { listChannels } from '../../../../utils/irc-mock'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, max-age=60')
  const query = getQuery(event)
  try {
    return {
      items: listChannels(
        await demoSession(event),
        getRouterParam(event, 'id')!,
        typeof query.q === 'string' ? query.q.slice(0, 100) : '',
        Number(query.limit) || 50,
      ),
    }
  } catch (error) {
    routeError(error)
  }
})
