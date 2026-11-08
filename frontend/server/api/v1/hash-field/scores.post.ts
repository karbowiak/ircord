export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  const account = await requireAccount(event)
  return submitHashFieldScore(account.id, await readBody(event))
})
