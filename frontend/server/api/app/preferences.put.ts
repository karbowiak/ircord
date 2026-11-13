export default defineEventHandler(async (event) => {
  noStore(event)
  const account = await accountId(event)
  const body = await readBody<{ preferences?: unknown }>(event)
  if (!body?.preferences || typeof body.preferences !== 'object')
    throw createError({ statusCode: 400, statusMessage: 'Send preferences to save.' })
  return writePreferences(account, body.preferences, event)
})
