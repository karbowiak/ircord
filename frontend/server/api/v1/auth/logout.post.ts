export default defineEventHandler(async (event) => {
  await logout(event)
  setResponseStatus(event, 204)
  return null
})
