/** Sends a test notification to every device of the account. */
export default defineEventHandler(async (event) => {
  const account = await requireAccount(event)
  await sendPush(account.id, {
    title: 'IRCord',
    body: 'Notifications are working on this device.',
    tag: 'test',
  })
  setResponseStatus(event, 204)
  return null
})
