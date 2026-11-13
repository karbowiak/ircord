import {
  createSession,
  findSession,
  getConversation,
  limits,
  sendMessage,
} from '../../../utils/demo-store'
import { sendRaw } from '../../../utils/irc-mock'
import { GoSocket, backendSocketURL, socketOrigin } from '../../../utils/go-socket'
import type { ClientEvent, ServerEvent } from '../../../../shared/types/chat'

export default defineWebSocketHandler({
  async upgrade(request) {
    const origin = request.headers.get('origin')
    if (origin && origin !== socketOrigin(request.url))
      throw new Response('Origin not allowed', { status: 403 })
    const backend = backendSocketURL()
    if (backend) {
      if (!origin) throw new Response('Origin required', { status: 403 })
      const bridge = new GoSocket(backend, request)
      try {
        await bridge.ready
        const context = request.context ?? {}
        context.goSocket = bridge
        return { headers: undefined, context }
      } catch (error) {
        bridge.close()
        throw error
      }
    }
    const token = request.headers
      .get('cookie')
      ?.split(';')
      .map((s) => s.trim())
      .find((s) => s.startsWith('ircord_session='))
      ?.slice('ircord_session='.length)
    const account = await accountForToken(token)
    if (!account) throw new Response('Sign in first', { status: 401 })
    const session = findSession(account.id) ?? createSession(account.id)
    if (session.listeners.size >= limits.connections)
      throw new Response('Too many open devices', { status: 429 })
    const context = request.context ?? {}
    context.sessionId = session.id
    return { headers: undefined, context }
  },
  open(peer) {
    if (peer.context.goSocket instanceof GoSocket) {
      peer.context.goSocket.bind(peer)
      return
    }
    const session = findSession(peer.context.sessionId as string)
    if (!session) {
      peer.close(1008, 'Session expired')
      return
    }
    const listener = (event: ServerEvent) => {
      try {
        peer.send(JSON.stringify(event))
      } catch {
        session.listeners.delete(listener)
      }
    }
    peer.context.listener = listener
    session.listeners.add(listener)
    peer.send({ v: 1, type: 'session.ready', cursor: session.bootstrap.cursor })
  },
  message(peer, frame) {
    if (peer.context.goSocket instanceof GoSocket) {
      peer.context.goSocket.send(frame.text())
      return
    }
    const session = findSession(peer.context.sessionId as string)
    if (!session) {
      peer.close(1008, 'Session expired')
      return
    }
    let clientId: string | undefined
    try {
      if (frame.text().length > 20_000) throw new Error('Message is too large.')
      const input = frame.json<ClientEvent>()
      if (input.v !== 1) throw new Error('Unsupported event version.')
      if (input.type === 'session.resume') {
        if (
          !Number.isSafeInteger(input.after) ||
          input.after < 0 ||
          input.after > session.bootstrap.cursor
        )
          throw new Error('Invalid event cursor.')
        if (input.after < (session.events[0]?.seq ?? session.bootstrap.cursor + 1) - 1)
          peer.send({
            v: 1,
            type: 'session.resync',
            reason: 'History changed while you were away.',
          })
        else for (const event of session.events) if (event.seq > input.after) peer.send(event)
      } else if (input.type === 'message.send') {
        clientId = input.clientId
        getConversation(session, input.conversationId)
        const existing = session.sent.has(input.clientId)
        const message = sendMessage(session, input)
        if (existing)
          peer.send({ v: 1, seq: session.bootstrap.cursor, type: 'message.created', data: message })
      } else if (input.type === 'typing.set') {
        // Real backends relay this as the `+typing` client tag; the demo has nobody to tell.
        getConversation(session, input.conversationId)
        if (!['active', 'paused', 'done'].includes(input.state))
          throw new Error('Invalid typing state.')
      } else if (input.type === 'raw.send') {
        clientId = input.clientId
        sendRaw(session, input.networkId, input.line)
      } else throw new Error('Unknown command.')
    } catch (error) {
      peer.send({
        v: 1,
        type: 'command.error',
        clientId,
        message: error instanceof Error ? error.message : 'Invalid command.',
      })
    }
  },
  close(peer) {
    if (peer.context.goSocket instanceof GoSocket) {
      peer.context.goSocket.close()
      return
    }
    const session = findSession(peer.context.sessionId as string)
    if (session && peer.context.listener)
      session.listeners.delete(peer.context.listener as (event: ServerEvent) => void)
  },
})
