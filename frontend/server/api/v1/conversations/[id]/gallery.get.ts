import { listGallery } from '../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    const q = getQuery(event)
    return listGallery(
      await demoSession(event),
      getRouterParam(event, 'id')!,
      typeof q.cursor === 'string' ? q.cursor : '',
      Number(q.limit) || 40,
    )
  } catch (error) {
    routeError(error)
  }
})
