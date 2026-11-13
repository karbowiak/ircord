export default defineEventHandler(async (event) => {
  noStore(event)
  const account = await accountId(event)
  const body = await readBody<{ settings?: unknown }>(event)
  if (!body?.settings || typeof body.settings !== 'object')
    throw createError({ statusCode: 400, statusMessage: 'Send appearance settings to save.' })
  return writeAppearance(account, body.settings, event)
})
