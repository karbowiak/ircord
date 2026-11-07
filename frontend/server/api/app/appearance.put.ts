export default defineEventHandler(async (event) => {
  noStore(event)
  const body = await readBody<{ settings?: unknown }>(event)
  if (!body?.settings || typeof body.settings !== 'object')
    throw createError({ statusCode: 400, statusMessage: 'Send appearance settings to save.' })
  return writeAppearance(await accountId(event), body.settings, event)
})
