export default defineEventHandler(async (event) => {
  await completeOnboarding(event)
  setResponseStatus(event, 204)
  return null
})
