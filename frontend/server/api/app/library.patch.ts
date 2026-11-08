import { recordUsage, sanitizeLibrary, usageOrder } from '../../../shared/library'

/**
 * `uses` ({ emojiId: times }) adds to the emoji usage counts on the server, so devices sending
 * at the same time never overwrite each other. `recent` lists those ids oldest pick first, so
 * recency survives batching. `skinTone` replaces the stored tone.
 */
export default defineEventHandler(async (event) => {
  noStore(event)
  const body = await readBody<{ uses?: unknown; recent?: unknown; skinTone?: unknown }>(event)
  const uses =
    body?.uses && typeof body.uses === 'object' ? (body.uses as Record<string, number>) : {}
  const recent = usageOrder(body?.recent, uses)
  if (Object.keys(uses).length > 100 || recent.length > 100)
    throw createError({ statusCode: 400, statusMessage: 'Too many emoji in one update.' })
  return updateLibrary(
    await accountId(event),
    (library) => {
      library.emojiUsage = recordUsage(library.emojiUsage, uses, new Date(), recent)
      if (body?.skinTone !== undefined)
        library.skinTone = sanitizeLibrary({ skinTone: body.skinTone }).skinTone
    },
    event,
  )
})
