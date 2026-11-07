/** Demo fixture only; the live middleware forwards address checks to Go. */
export default defineEventHandler(async (event) => {
  await demoSession(event)
  const input = await readBody(event)
  if (
    !input ||
    typeof input.host !== 'string' ||
    !Number.isInteger(input.port) ||
    input.port < 1 ||
    input.port > 65535 ||
    typeof input.tls !== 'boolean'
  )
    throw createError({ statusCode: 400, statusMessage: 'Enter a valid server address and port.' })
  return input.host.endsWith('.invalid')
    ? { reachable: false, detail: 'The server address could not be found. Check the hostname.' }
    : { reachable: true, detail: 'Server address verified. Sign-in is checked when you connect.' }
})
