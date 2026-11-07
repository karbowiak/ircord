import { removeConversation } from '../../../utils/irc-mock'

/** Leaves a channel (PART) or closes a direct conversation. */
export default defineEventHandler(async (event) => {
  try {
    removeConversation(await demoSession(event), getRouterParam(event, 'id')!)
    setResponseStatus(event, 204)
    return null
  } catch (error) {
    routeError(error)
  }
})
