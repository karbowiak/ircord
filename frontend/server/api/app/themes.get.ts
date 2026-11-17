export default defineEventHandler(async (event) => {
  noStore(event)
  return readThemes(await accountId(event), event)
})
