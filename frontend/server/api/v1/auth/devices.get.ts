/** This account's signed-in devices (#85). */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return listDevices(event)
})
