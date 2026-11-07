export default defineEventHandler(async (event) => {
  noStore(event)
  return readPreferences(await accountId(event), event)
})
