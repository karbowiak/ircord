/**
 * Sends a test notification to the requesting device only, like the backend: 409 when nothing
 * there accepted it (no registration, or it just turned out gone), 503 when delivery failed.
 */
export default defineEventHandler(async (event) => {
  const account = await requireAccount(event)
  const { delivered, failed } = await sendPush(
    account.id,
    { title: 'IRCord', body: 'Notifications are working on this device.', tag: 'test' },
    currentDevice(event),
  )
  if (!delivered && failed)
    throw createError({
      statusCode: 503,
      statusMessage: 'Test notification could not be delivered.',
    })
  if (!delivered)
    throw createError({
      statusCode: 409,
      statusMessage: 'This device has no push registration. Set up notifications again.',
    })
  setResponseStatus(event, 204)
  return null
})
