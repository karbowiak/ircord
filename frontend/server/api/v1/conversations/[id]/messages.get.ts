import { history } from '../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const q = getQuery(event)
    return history(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      typeof q.before === 'string' ? q.before : undefined,
      Number(q.limit) || 40,
      typeof q.around === 'string' ? q.around : undefined,
    )
  } catch (error) {
    routeError(error)
  }
})
