import type { ServicesRole } from '../../../../../../shared/types/services'
import * as services from '../../../../../utils/services-mock'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ mode?: unknown }>(event)
  return services.chanservSetAccess(
    await demoSession(event),
    getRouterParam(event, 'id')!,
    decodeURIComponent(getRouterParam(event, 'account')!),
    String(body?.mode ?? '') as ServicesRole,
  )
})
