import type { ConnectionInfo, Network } from '../../shared/types/chat'

/** A fictional handshake for the isolated Nitro demo, never used in Go mode. */
export function demoConnectionInfo(network: Network, startedAt = Date.now()): ConnectionInfo {
  const f = network.features
  const negotiated = ['echo-message', 'server-time', 'cap-notify']
  if (f.replies || f.reactions || f.typing || f.edit) negotiated.push('message-tags')
  if (f.multiline || f.history) negotiated.push('batch')
  if (f.multiline) negotiated.push('draft/multiline')
  if (f.history) negotiated.push('draft/chathistory')
  if (f.redact) negotiated.push('draft/message-redaction')
  if (f.readMarkers) negotiated.push('draft/read-marker')
  if (f.metadata.length) negotiated.push('draft/metadata-3')
  if (f.setname) negotiated.push('setname')
  const offered = Object.fromEntries(
    [...new Set([...negotiated, 'account-tag', 'away-notify', 'multi-prefix', 'sasl'])].map(
      (cap) => [cap, ''],
    ),
  )
  if (f.multiline)
    offered['draft/multiline'] =
      `max-bytes=${f.multiline.maxBytes},max-lines=${f.multiline.maxLines}`
  offered.sasl = 'PLAIN,EXTERNAL'
  return {
    networkId: network.id,
    state: network.status,
    serverName: network.host,
    connectedAt: new Date(startedAt).toISOString(),
    isupport: { ...network.isupport },
    offeredCapabilities: offered,
    negotiatedCapabilities: network.status === 'connecting' ? [] : negotiated,
    motd: [`Welcome to the ${network.name} demonstration server.`],
    ...(network.statusDetail ? { lastError: network.statusDetail } : {}),
    ...(network.tls
      ? {
          tls: {
            version: 'TLS 1.3',
            cipher: 'TLS_AES_256_GCM_SHA384',
            verified: true,
            serverName: network.host,
          },
        }
      : {}),
  }
}
