import { patchConversation } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return patchConversation(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      await readBody(event),
    )
  } catch (error) {
    routeError(error)
  }
})
