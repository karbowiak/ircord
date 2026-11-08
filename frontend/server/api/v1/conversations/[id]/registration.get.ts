import * as services from '../../../../utils/services-mock'

/** Whether a channel is registered with ChanServ, and to whom. */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  return services.chanservInfo(await demoSession(event), getRouterParam(event, 'id')!)
})
