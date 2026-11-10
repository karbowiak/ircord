import { emit } from '../../../../utils/demo-store'
import { wireConversation } from '../../../../../shared/utils/conversation'

export default defineEventHandler(async (event) => {
  try {
    const session = await demoSession(event)
    const conversation = session.bootstrap.conversations.find(
      (item) => item.party?.id === getRouterParam(event, 'id'),
    )
    const party = conversation?.party
    if (!conversation || !party) throw new Error('Group conversation not found.')
    const network = session.bootstrap.networks.find((item) => item.id === conversation.networkId)
    const owner = party.members.find((member) => member.id === party.ownerId)
    if (
      !owner ||
      !(
        owner.backendAccountId === session.id ||
        (owner.servicesAccount && owner.servicesAccount === network?.account)
      )
    )
      throw new Error('Only the group owner can convert it.')
    emit(session, {
      type: 'party.updated',
      data: { ...party, state: 'converted', revision: party.revision + 1 },
    })
    delete conversation.party
    emit(session, { type: 'conversation.updated', data: wireConversation(conversation) })
    return { status: 'converted', conversationId: conversation.id, warnings: [] }
  } catch (error) {
    routeError(error)
  }
})
