import { demoJoinFromGuide } from '../../../../../utils/demo-channel-guide'

/** Joins the chosen channel guide channels in one request (#258). */
export default defineEventHandler(async (event) => {
  try {
    const body = await readBody<{ channels?: unknown }>(event)
    const channels = Array.isArray(body?.channels)
      ? body.channels.filter((c): c is string => typeof c === 'string').slice(0, 50)
      : []
    return demoJoinFromGuide(await demoSession(event), getRouterParam(event, 'id')!, channels)
  } catch (error) {
    routeError(error)
  }
})
