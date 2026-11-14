import {
  ForbiddenError,
  previewRecentMessages,
  type RecentRange,
} from '../../../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const range = getQuery(event).range
    return previewRecentMessages(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      decodeURIComponent(getRouterParam(event, 'memberId')!),
      (typeof range === 'string' ? range : '24h') as RecentRange,
    )
  } catch (error) {
    if (error instanceof ForbiddenError)
      throw createError({ statusCode: 403, statusMessage: error.message })
    routeError(error)
  }
})
