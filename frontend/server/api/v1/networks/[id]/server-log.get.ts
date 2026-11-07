import { serverLog } from '../../../../utils/irc-mock'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  const query = getQuery(event)
  try {
    return serverLog(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      typeof query.before === 'string' ? query.before : undefined,
      Number(query.limit) || 100,
    )
  } catch (error) {
    routeError(error)
  }
})
