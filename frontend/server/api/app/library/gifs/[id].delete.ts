export default defineEventHandler(async (event) => {
  noStore(event)
  const id = getRouterParam(event, 'id', { decode: true })
  return updateLibrary(
    await accountId(event),
    (library) => {
      library.gifs = library.gifs.filter((item) => item.id !== id)
    },
    event,
  )
})
