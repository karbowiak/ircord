export default defineEventHandler(async (event) => {
  noStore(event)
  const body = await readBody<{ preferences?: unknown }>(event)
  if (!body?.preferences || typeof body.preferences !== 'object')
    throw createError({ statusCode: 400, statusMessage: 'Send preferences to save.' })
  return writePreferences(await accountId(event), body.preferences, event)
})
