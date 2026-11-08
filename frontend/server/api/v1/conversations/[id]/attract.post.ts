import { attract } from '../../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return attract(await demoSession(event), getRouterParam(event, 'id')!)
  } catch (error) {
    routeError(error)
  }
})
