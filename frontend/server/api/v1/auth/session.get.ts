/** The signed-in account, or `{ account: null }` when signed out (not an error). */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return { account: await currentAccount(event) }
})
