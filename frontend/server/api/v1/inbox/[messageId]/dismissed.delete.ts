import { setInboxDismissed } from '../../../../utils/demo-inbox'
export default defineEventHandler(async (event) => {
  try {
    setInboxDismissed(await demoSession(event), decodeURIComponent(getRouterParam(event, 'messageId')!), false)
    setResponseStatus(event, 204)
    return null
  } catch (error) {
    routeError(error)
  }
})
