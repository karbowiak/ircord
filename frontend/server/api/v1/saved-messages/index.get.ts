import { listSaved } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const q = getQuery(event)
    return listSaved(
      await demoSession(event),
      typeof q.cursor === 'string' ? q.cursor : '',
      typeof q.q === 'string' ? q.q.slice(0, 200) : '',
      Number(q.limit) || 30,
    )
  } catch (error) {
    routeError(error)
  }
})
