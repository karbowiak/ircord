import { listChannels } from '../../../../../utils/irc-mock'

export default defineEventHandler(async (event) => {
  try {
    const name = String(getQuery(event).name ?? '').toLowerCase()
    const channels = listChannels(await demoSession(event), getRouterParam(event, 'id')!, name, 100)
    return {
      status: channels.some((channel) => channel.name.toLowerCase() === name)
        ? 'unavailable'
        : 'unknown',
    }
  } catch (error) {
    routeError(error)
  }
})
