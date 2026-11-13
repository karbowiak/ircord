import { libraryLimits, savedGif } from '../../../../shared/library'
import { validGif } from '../../../../shared/utils/media'

export default defineEventHandler(async (event) => {
  noStore(event)
  const account = await accountId(event)
  const gif = await readBody(event)
  if (!validGif(gif))
    throw createError({ statusCode: 400, statusMessage: 'This GIF can’t be saved.' })
  return updateLibrary(
    account,
    (library) => {
      library.gifs = [savedGif(gif), ...library.gifs.filter((item) => item.id !== gif.id)].slice(
        0,
        libraryLimits.gifs,
      )
    },
    event,
  )
})
