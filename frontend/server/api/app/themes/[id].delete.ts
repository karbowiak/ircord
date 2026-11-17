import { isThemeId } from '../../../../shared/themes'

/** Removes one theme. Its background image is no longer named anywhere, so the hourly cleanup removes it later. */
export default defineEventHandler(async (event) => {
  noStore(event)
  const account = await accountId(event)
  const id = getRouterParam(event, 'id')
  if (!isThemeId(id)) throw createError({ statusCode: 400, statusMessage: 'Unknown theme.' })
  return updateThemes(
    account,
    (library) => void (library.themes = library.themes.filter((theme) => theme.id !== id)),
    event,
  )
})
