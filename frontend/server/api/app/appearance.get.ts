export default defineEventHandler(async (event) => {
  noStore(event)
  return readAppearance(await accountId(event), event)
})
