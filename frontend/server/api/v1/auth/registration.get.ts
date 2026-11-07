/** Seeded demo policy. Live requests are handled by the Go auth middleware. */
export default defineEventHandler((event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  return { registrationEnabled: true, registrationKeyRequired: false }
})
