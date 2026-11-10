import { getNetwork } from '../../../../utils/demo-store'

export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  return getNetwork(session, getRouterParam(event, 'id')!).profile
})
