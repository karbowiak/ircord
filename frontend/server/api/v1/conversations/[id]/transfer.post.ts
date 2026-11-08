import * as services from '../../../../utils/services-mock'

export default defineEventHandler(async (event) => {
  const body = await readBody<{ account?: unknown }>(event)
  if (typeof body?.account !== 'string' || !body.account.trim())
    throw createError({ statusCode: 400, statusMessage: 'Choose an account.' })
  return services.chanservTransfer(
    await demoSession(event),
    getRouterParam(event, 'id')!,
    body.account.trim(),
  )
})
