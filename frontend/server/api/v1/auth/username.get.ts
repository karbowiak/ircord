/** Whether a username can be used: valid, and free on IRCord and irc.ircord.com. */
export default defineEventHandler((event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return usernameCheck(getQuery(event).name)
})
