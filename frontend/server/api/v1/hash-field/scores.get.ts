export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  await requireAccount(event)
  return hashFieldTable()
})
