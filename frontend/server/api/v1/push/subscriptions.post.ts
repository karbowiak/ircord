export default defineEventHandler(async (event) => {
  const account = await requireAccount(event)
  const body = await readBody<{ subscription?: unknown; device?: string }>(event)
  if (!validSubscription(body?.subscription))
    throw createError({ statusCode: 400, statusMessage: 'That push subscription isn’t valid.' })
  await addSubscription(
    account.id,
    body.subscription,
    typeof body.device === 'string' ? body.device : undefined,
  )
  setResponseStatus(event, 204)
  return null
})
