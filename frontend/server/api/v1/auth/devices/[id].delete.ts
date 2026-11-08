/** Signs one of this account's devices out (#85). */
export default defineEventHandler(async (event) => {
  await signOutDevice(event, getRouterParam(event, 'id') ?? '')
  setResponseStatus(event, 204)
  return null
})
