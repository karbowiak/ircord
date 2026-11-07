/** Uploaded files are private to the account in the demo; Go may serve them publicly. */
export default defineEventHandler(async (event) => {
  const upload = (await demoSession(event)).uploads.get(getRouterParam(event, 'id')!)
  if (!upload)
    throw createError({ statusCode: 404, statusMessage: 'This file is no longer available.' })
  setHeader(event, 'Content-Type', upload.contentType)
  setHeader(event, 'Cache-Control', 'private, max-age=86400, immutable')
  setHeader(event, 'X-Content-Type-Options', 'nosniff')
  setHeader(
    event,
    'Content-Disposition',
    `${upload.contentType.startsWith('image/') || upload.contentType === 'video/mp4' ? 'inline' : 'attachment'}; filename="${upload.name}"`,
  )
  return upload.bytes
})
