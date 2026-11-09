export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  try {
    await demoSession(event)
    return { serverTime: new Date().toISOString() }
  } catch (error) {
    routeError(error)
  }
})
