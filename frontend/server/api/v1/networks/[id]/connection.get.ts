import { getNetwork } from '../../../../utils/demo-store'
import { demoConnectionInfo } from '../../../../utils/connection-mock'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  try {
    const session = await demoSession(event)
    return demoConnectionInfo(
      getNetwork(session, getRouterParam(event, 'id')!),
      session.windowStart,
    )
  } catch (error) {
    routeError(error)
  }
})
