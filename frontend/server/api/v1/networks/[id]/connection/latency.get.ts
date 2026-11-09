import { getNetwork } from '../../../../../utils/demo-store'
import { demoLatency } from '../../../../../utils/connection-mock'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  try {
    const session = await demoSession(event)
    // The demo pretends the connection is an hour old, so the chart is full on first look.
    return demoLatency(
      getNetwork(session, getRouterParam(event, 'id')!),
      session.windowStart - 60 * 60_000,
    )
  } catch (error) {
    routeError(error)
  }
})
