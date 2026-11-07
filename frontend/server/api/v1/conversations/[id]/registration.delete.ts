import * as services from '../../../../utils/services-mock'

export default defineEventHandler(async (event) => {
  services.chanservUnregister(await demoSession(event), getRouterParam(event, 'id')!)
  setResponseStatus(event, 204)
  return null
})
