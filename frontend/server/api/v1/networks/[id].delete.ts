import { removeNetwork } from '../../../utils/demo-store'

export default defineEventHandler(async (event) => {
  try {
    removeNetwork(await demoSession(event), getRouterParam(event, 'id')!)
    setResponseStatus(event, 204)
    return null
  } catch (error) {
    routeError(error)
  }
})
