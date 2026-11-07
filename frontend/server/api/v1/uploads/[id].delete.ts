import { deleteUpload } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    deleteUpload(await demoSession(event), getRouterParam(event, 'id')!)
    return null
  } catch (error) {
    routeError(error)
  }
})
