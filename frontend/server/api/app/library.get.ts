export default defineEventHandler(async (event) => {
  noStore(event)
  return readLibrary(await accountId(event), event)
})
