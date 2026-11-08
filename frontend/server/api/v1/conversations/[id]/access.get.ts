import * as services from '../../../../utils/services-mock'

/** Persistent roles (ChanServ AMODE) by account. */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  return services.chanservAccess(await demoSession(event), getRouterParam(event, 'id')!)
})
