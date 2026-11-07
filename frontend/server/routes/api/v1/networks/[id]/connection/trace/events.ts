import { GoSocket, backendSocketURL, socketOrigin } from '../../../../../../../utils/go-socket'

/** Diagnostic streams retain the same authenticated, origin-checked proxy. */
export default defineWebSocketHandler({
  async upgrade(request) {
    const origin = request.headers.get('origin')
    if (!origin || origin !== socketOrigin(request.url))
      throw new Response('Origin not allowed', { status: 403 })
    const backend = backendSocketURL()
    if (!backend) throw new Response('Diagnostics require the Go backend', { status: 404 })
    const bridge = new GoSocket(backend, request)
    await bridge.ready
    request.context.goSocket = bridge
  },
  open(peer) { if (peer.context.goSocket instanceof GoSocket) peer.context.goSocket.bind(peer) },
  message(peer) { peer.close(1008, 'Diagnostic streams are receive-only') },
  close(peer) { if (peer.context.goSocket instanceof GoSocket) peer.context.goSocket.close() },
})
