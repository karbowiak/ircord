export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  return writeNotificationSettings((await requireAccount(event)).id, await readBody(event))
})
