/** Sends a test notification to the requesting device only; 409 when it has no registration. */
export default defineEventHandler(async (event) => {
  const account = await requireAccount(event)
  const sent = await sendPush(
    account.id,
    { title: 'IRCord', body: 'Notifications are working on this device.', tag: 'test' },
    currentDevice(event),
  )
  if (!sent)
    throw createError({
      statusCode: 409,
      statusMessage: 'This device has no push registration. Set up notifications again.',
    })
  setResponseStatus(event, 204)
  return null
})
