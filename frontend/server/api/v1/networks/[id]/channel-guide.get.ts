import { demoChannelGuide } from '../../../../utils/demo-channel-guide'

/** The server's channel guide (#258); only the demo's stand-in home network has one. */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  try {
    return demoChannelGuide(await demoSession(event), getRouterParam(event, 'id')!)
  } catch (error) {
    routeError(error)
  }
})
