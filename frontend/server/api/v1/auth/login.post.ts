export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return login(event, (await readBody<{ key?: unknown }>(event))?.key)
})
