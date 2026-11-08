import * as services from '../../../../utils/services-mock'

export default defineEventHandler(async (event) =>
  services.chanservRegister(await demoSession(event), getRouterParam(event, 'id')!),
)
