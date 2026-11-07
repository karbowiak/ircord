import { recordUsage, sanitizeLibrary } from '../../../shared/library'

/**
 * `uses` ({ emojiId: times }) adds to the emoji usage counts on the server, so devices sending
 * at the same time never overwrite each other. `skinTone` replaces the stored tone.
 */
export default defineEventHandler(async (event) => {
  noStore(event)
  const body = await readBody<{ uses?: unknown; skinTone?: unknown }>(event)
  const uses =
    body?.uses && typeof body.uses === 'object' ? (body.uses as Record<string, number>) : {}
  if (Object.keys(uses).length > 100)
    throw createError({ statusCode: 400, statusMessage: 'Too many emoji in one update.' })
  return updateLibrary(
    await accountId(event),
    (library) => {
      library.emojiUsage = recordUsage(library.emojiUsage, uses)
      if (body?.skinTone !== undefined)
        library.skinTone = sanitizeLibrary({ skinTone: body.skinTone }).skinTone
    },
    event,
  )
})
