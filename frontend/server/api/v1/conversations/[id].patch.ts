import { ChannelSettingError, patchConversation } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return patchConversation(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      await readBody(event),
    )
  } catch (error) {
    if (error instanceof ChannelSettingError)
      throw createError({ statusCode: 422, statusMessage: error.message })
    routeError(error)
  }
})
