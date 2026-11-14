import {
  deleteRecentMessages,
  ForbiddenError,
  type RecentRange,
} from '../../../../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const body = await readBody<{ range?: RecentRange }>(event)
    setResponseStatus(event, 202)
    return deleteRecentMessages(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      decodeURIComponent(getRouterParam(event, 'memberId')!),
      body?.range ?? '24h',
    )
  } catch (error) {
    if (error instanceof ForbiddenError)
      throw createError({ statusCode: 403, statusMessage: error.message })
    routeError(error)
  }
})
