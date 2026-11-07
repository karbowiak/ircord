export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  return readNotificationSettings((await requireAccount(event)).id)
})
