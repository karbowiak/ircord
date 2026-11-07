import { emit, getNetwork, updateNetwork } from '../../../utils/demo-store'
import { serverLine } from '../../../utils/irc-mock'

/**
 * `{ status }` connects or disconnects; a full `NetworkInput` edits the connection settings.
 * Connecting goes through `connecting` first, like a real registration, and fails with a reason
 * when SASL has no saved password.
 */
export default defineEventHandler(async (event) => {
  const session = await demoSession(event)
  const id = getRouterParam(event, 'id')!
  const body = await readBody(event)
  try {
    if (body && 'status' in body && Object.keys(body).length === 1) {
      if (body.status !== 'connected' && body.status !== 'disconnected')
        throw new Error('Invalid connection status.')
      const network = getNetwork(session, id)
      network.statusDetail = undefined
      if (body.status === 'disconnected') {
        network.status = 'disconnected'
        emit(session, { type: 'network.updated', data: network })
        return network
      }
      network.status = 'connecting'
      emit(session, { type: 'network.updated', data: network })
      const timer = setTimeout(() => {
        session.timers.delete(timer)
        if (network.status !== 'connecting') return
        const saslWithoutPassword =
          network.auth.method === 'sasl-plain' && !network.auth.passwordSet
        network.status = saslWithoutPassword ? 'error' : 'connected'
        network.statusDetail = saslWithoutPassword
          ? 'SASL login failed: no password is saved.'
          : undefined
        serverLine(session, network.id, {
          kind: saslWithoutPassword ? 'error' : 'numeric',
          command: saslWithoutPassword ? '904' : '001',
          source: network.host,
          params: [network.nick],
          text: saslWithoutPassword
            ? 'SASL authentication failed'
            : `Welcome back to ${network.name}, ${network.nick}`,
        })
        emit(session, { type: 'network.updated', data: network })
      }, 900)
      session.timers.add(timer)
      return network
    }
    return updateNetwork(session, id, body)
  } catch (error) {
    routeError(error)
  }
})
