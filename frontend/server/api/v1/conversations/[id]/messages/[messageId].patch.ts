import { ForbiddenError, editMessage } from '../../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  const body = await readBody<{ text?: unknown }>(event)
  try {
    return editMessage(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      decodeURIComponent(getRouterParam(event, 'messageId')!),
      typeof body?.text === 'string' ? body.text : '',
    )
  } catch (error) {
    if (error instanceof ForbiddenError)
      throw createError({ statusCode: 403, statusMessage: error.message })
    routeError(error)
  }
})
