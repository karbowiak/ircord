export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return (await demoSession(event)).bootstrap
})
