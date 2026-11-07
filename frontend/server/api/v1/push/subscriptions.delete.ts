export default defineEventHandler(async (event) => {
  const account = await requireAccount(event)
  const body = await readBody<{ endpoint?: unknown }>(event)
  if (typeof body?.endpoint === 'string') await removeSubscription(account.id, body.endpoint)
  setResponseStatus(event, 204)
  return null
})
