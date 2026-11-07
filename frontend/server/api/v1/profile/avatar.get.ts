export default defineEventHandler(async (event) => {
  const avatar = (await demoSession(event)).profileAvatar
  if (!avatar) throw createError({ statusCode: 404, statusMessage: 'No profile image is set.' })
  setHeader(event, 'Content-Type', avatar.contentType)
  setHeader(event, 'Cache-Control', 'private, no-store')
  setHeader(event, 'X-Content-Type-Options', 'nosniff')
  return Buffer.from(avatar.bytes)
})
