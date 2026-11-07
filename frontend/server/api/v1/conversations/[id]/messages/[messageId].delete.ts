import { ForbiddenError, redactMessage } from '../../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return redactMessage(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      decodeURIComponent(getRouterParam(event, 'messageId')!),
    )
  } catch (error) {
    if (error instanceof ForbiddenError)
      throw createError({ statusCode: 403, statusMessage: error.message })
    routeError(error)
  }
})
