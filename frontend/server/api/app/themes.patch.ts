/** The account's own "Use custom CSS" switch. Themes stay stored either way. */
export default defineEventHandler(async (event) => {
  noStore(event)
  const account = await accountId(event)
  const body = await readBody<{ customCss?: unknown }>(event)
  if (typeof body?.customCss !== 'boolean')
    throw createError({ statusCode: 400, statusMessage: 'Send whether to use custom CSS.' })
  const customCss = body.customCss
  return updateThemes(account, (library) => void (library.customCss = customCss), event)
})
