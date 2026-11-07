import * as services from '../../../../../utils/services-mock'

export default defineEventHandler(async (event) =>
  services.chanservRemoveAccess(
    await demoSession(event),
    getRouterParam(event, 'id')!,
    decodeURIComponent(getRouterParam(event, 'account')!),
  ),
)
