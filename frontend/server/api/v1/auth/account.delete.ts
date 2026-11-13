export default defineEventHandler(async (event) => {
  await deleteAccount(event, (await readBody(event)) ?? {})
  setResponseStatus(event, 204)
  return null
})
