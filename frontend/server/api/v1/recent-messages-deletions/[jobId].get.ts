import { recentMessagesDeletion } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return recentMessagesDeletion(await demoSession(event), getRouterParam(event, 'jobId')!)
  } catch (error) {
    routeError(error)
  }
})
