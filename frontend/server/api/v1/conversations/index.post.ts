import { addConversation } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  try {
    return addConversation(await demoSession(event), await readBody(event))
  } catch (error) {
    routeError(error)
  }
})
