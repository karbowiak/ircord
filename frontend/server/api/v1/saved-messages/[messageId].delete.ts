import { unsaveMessage } from '../../../utils/demo-store'
export default defineEventHandler(async (event) => {
  unsaveMessage(await demoSession(event), decodeURIComponent(getRouterParam(event, 'messageId')!))
  setResponseStatus(event, 204)
  return null
})
