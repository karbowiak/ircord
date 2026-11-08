/** Signs in with a username and password, or (until keys are retired) an account key. */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return login(event, await readBody(event))
})
