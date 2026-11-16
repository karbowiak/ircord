/** Demo channels have no place in a channel guide; operators ask on a real server (#258). */
export default defineEventHandler((event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  return { available: false, operator: false, status: 'none', groups: [] }
})
