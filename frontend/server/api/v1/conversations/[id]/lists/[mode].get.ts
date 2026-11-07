import { getConversation } from '../../../../../utils/demo-store'
import { channelList } from '../../../../../utils/irc-mock'
import { parseChanmodes } from '../../../../../../shared/utils/channel-modes'

/** A channel's ban, exception, invite-exception or quiet list (any list mode in CHANMODES). */
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'private, no-store')
  const session = await demoSession(event)
  try {
    const c = getConversation(session, getRouterParam(event, 'id')!)
    const network = session.bootstrap.networks.find((n) => n.id === c.networkId)
    const mode = getRouterParam(event, 'mode') ?? ''
    if (
      c.kind !== 'channel' ||
      mode.length !== 1 ||
      !parseChanmodes(network?.isupport).list.includes(mode)
    )
      throw new Error('That isn’t a list mode on this network.')
    return { items: channelList(session, c, mode) }
  } catch (error) {
    routeError(error)
  }
})
