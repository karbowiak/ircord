import {
  byteLength,
  isThemeId,
  sanitizeTheme,
  themeLimits,
  type Theme,
} from '../../../../shared/themes'

/**
 * Saves one theme and leaves the others alone. `expected` is the `updatedAt` the editor started
 * from; if the stored theme changed since, the save is refused with 412 and the stored copy, so the
 * person can keep theirs (`force`) or load the other one.
 */
export default defineEventHandler(async (event) => {
  noStore(event)
  const account = await accountId(event)
  const id = getRouterParam(event, 'id')
  if (!isThemeId(id)) throw createError({ statusCode: 400, statusMessage: 'Unknown theme.' })
  const body = await readBody<{ theme?: unknown; expected?: unknown; force?: unknown }>(event)
  // Refuse rather than let sanitizing cut the CSS off mid-rule.
  const css = (body?.theme as { css?: unknown } | undefined)?.css
  if (typeof css === 'string' && byteLength(css) > themeLimits.css)
    throw createError({
      statusCode: 400,
      statusMessage: `A theme can have up to ${themeLimits.css / 1024} KB of CSS.`,
    })
  const incoming = sanitizeTheme({
    ...(body?.theme && typeof body.theme === 'object' ? body.theme : {}),
    id,
    updatedAt: new Date().toISOString(),
  })
  if (!incoming) throw createError({ statusCode: 400, statusMessage: 'Send a theme to save.' })
  const expected = typeof body?.expected === 'string' ? body.expected : null
  return updateThemes(
    account,
    (library) => {
      const index = library.themes.findIndex((theme) => theme.id === id)
      const stored: Theme | undefined = library.themes[index]
      if (stored && body?.force !== true && stored.updatedAt !== expected)
        throw createError({
          statusCode: 412,
          statusMessage: 'This theme was changed on another device.',
          data: { theirs: stored },
        })
      if (stored) library.themes[index] = incoming
      else if (library.themes.length >= themeLimits.themes)
        throw createError({
          statusCode: 400,
          statusMessage: `You can keep up to ${themeLimits.themes} themes. Delete one to add another.`,
        })
      else library.themes.push(incoming)
    },
    event,
  )
})
