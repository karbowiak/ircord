/** Creates a username and password account (and, live, its irc.ircord.com account). */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  return signup(event, (await readBody(event)) ?? {})
})
