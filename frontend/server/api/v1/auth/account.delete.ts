export default defineEventHandler(async (event) => {
  await deleteAccount(event)
  setResponseStatus(event, 204)
  return null
})
