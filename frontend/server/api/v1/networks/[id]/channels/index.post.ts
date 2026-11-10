import { addConversation } from '../../../../../utils/demo-store'
import { listChannels } from '../../../../../utils/irc-mock'

export default defineEventHandler(async (event) => {
  try {
    const session = await demoSession(event)
    const networkId = getRouterParam(event, 'id')!
    const input = await readBody<{
      name: string
      topic?: string
      flags?: string
      modeParams?: Record<string, string>
    }>(event)
    const name = input.name.trim().replace(/^#/, '')
    if (
      listChannels(session, networkId, name, 100).some(
        (channel) => channel.name.toLowerCase() === `#${name.toLowerCase()}`,
      )
    )
      throw new Error('This channel name is unavailable.')
    const conversation = addConversation(session, { networkId, name })
    conversation.topic = input.topic ?? ''
    const modes = new Set(conversation.modes ?? '')
    let adding = true
    for (const mode of input.flags ?? '') {
      if (mode === '+') adding = true
      else if (mode === '-') adding = false
      else if (adding) modes.add(mode)
      else modes.delete(mode)
    }
    for (const mode of Object.keys(input.modeParams ?? {})) modes.add(mode)
    conversation.modes = [...modes].sort().join('')
    conversation.modeParams = Object.fromEntries(
      Object.entries(input.modeParams ?? {}).filter(([mode]) => mode !== 'k'),
    )
    return { conversation, registered: true, configured: true, warnings: [] }
  } catch (error) {
    routeError(error)
  }
})
