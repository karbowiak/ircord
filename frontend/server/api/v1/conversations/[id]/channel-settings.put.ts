import { ForbiddenError, updateChannelSettings } from '../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return updateChannelSettings(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      await readBody(event),
    )
  } catch (error) {
    if (error instanceof ForbiddenError)
      throw createError({ statusCode: 403, statusMessage: error.message })
    routeError(error)
  }
})
