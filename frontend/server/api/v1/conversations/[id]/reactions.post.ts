import { toggleReaction } from '../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const body = await readBody(event)
    return toggleReaction(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      body.messageId,
      body.emoji,
    )
  } catch (error) {
    routeError(error)
  }
})
