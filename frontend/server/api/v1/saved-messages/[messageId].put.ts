import { saveMessage } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const body = await readBody(event)
    return saveMessage(
      await demoSession(event),
      String(body?.conversationId ?? ''),
      decodeURIComponent(getRouterParam(event, 'messageId')!),
    )
  } catch (error) {
    routeError(error)
  }
})
