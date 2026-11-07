import { addNetwork } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return addNetwork(await demoSession(event), await readBody(event))
  } catch (error) {
    routeError(error)
  }
})
