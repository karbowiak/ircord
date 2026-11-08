/** A key account chooses its username and password. */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return setCredentials(event, (await readBody(event)) ?? {})
})
