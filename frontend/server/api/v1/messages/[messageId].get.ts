import { locateMessage } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return locateMessage(
      await demoSession(event),
      decodeURIComponent(getRouterParam(event, 'messageId')!),
    )
  } catch (error) {
    routeError(error)
  }
})
