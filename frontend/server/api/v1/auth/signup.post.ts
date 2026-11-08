/** Creates a username and password account (live, also its account on the associated IRC server). */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const result = await signup(event, (await readBody(event)) ?? {})
  setResponseStatus(event, 201)
  return result
})
