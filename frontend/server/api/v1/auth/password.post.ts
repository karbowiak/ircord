export default defineEventHandler(async (event) => {
  await changePassword(event, (await readBody(event)) ?? {})
  setResponseStatus(event, 204)
  return null
})
