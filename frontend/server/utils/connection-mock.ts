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
  if (f.edit) negotiated.push('account-tag')
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
    ...(network.tls ? { tls: demoTLS(network, startedAt) } : {}),
  }
}

/** A plausible certificate; one network shows a weaker setup so the grades differ. */
function demoTLS(network: Network, startedAt: number): NonNullable<ConnectionInfo['tls']> {
  const day = 86_400_000
  const older = /oftc/i.test(network.host)
  return {
    version: older ? 'TLS 1.2' : 'TLS 1.3',
    cipher: older ? 'TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384' : 'TLS_AES_256_GCM_SHA384',
    verified: true,
    serverName: network.host,
    subject: network.host,
    issuer: older ? 'GlobalSign RSA OV SSL CA 2018' : "Let's Encrypt E6",
    notBefore: new Date(startedAt - 30 * day).toISOString(),
    notAfter: new Date(startedAt + 60 * day).toISOString(),
    keyAlgorithm: older ? 'RSA' : 'ECDSA',
    keyBits: older ? 2048 : 256,
    signatureAlgorithm: older ? 'SHA256-RSA' : 'ECDSA-SHA384',
    forwardSecrecy: true,
    grade: older ? 'A-' : 'A+',
    gradeNotes: older
      ? [
          'Uses TLS 1.2. TLS 1.3 is faster and removes older, weaker options.',
          'The certificate uses a 2048-bit RSA key. A larger RSA key or an ECDSA key earns A+.',
        ]
      : [],
  }
}

/**
 * Keep-alive round trips for the demo: a sample every 20 seconds over the last half hour (or
 * since the session began), wobbling around a per-network baseline with the odd spike.
 */
export function demoLatency(network: Network, startedAt: number, now = Date.now()) {
  const interval = 20_000
  const window = 30 * 60_000
  const base = 18 + ([...network.id].reduce((sum, ch) => sum + ch.charCodeAt(0), 0) % 70)
  const samples = []
  const first = Math.max(startedAt + 30_000, now - window)
  for (let at = Math.ceil(first / interval) * interval; at <= now; at += interval) {
    const step = at / interval
    const wobble = Math.sin(step / 7) * base * 0.18 + Math.sin(step * 1.7) * 3
    const spike = step % 37 === 0 ? base * 1.8 : 0
    samples.push({
      at: new Date(at).toISOString(),
      ms: Math.round((base + wobble + spike) * 10) / 10,
    })
  }
  return {
    networkId: network.id,
    intervalSeconds: interval / 1000,
    windowSeconds: window / 1000,
    samples,
  }
}
