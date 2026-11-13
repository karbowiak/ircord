import type { NetworkInput } from './types/chat'

export interface NetworkPreset {
  name: string
  host: string
  port: number
  tls: boolean
  authMethod: NetworkInput['auth']['method']
  autoJoin: string[]
  color: 'lavender' | 'mint' | 'peach' | 'blue' | 'yellow'
  initial: string
  passwordProvided: boolean
}
const methods = new Set<NetworkPreset['authMethod']>(['none', 'server-password', 'sasl-plain', 'nickserv'])
const defaults = [
  { name: 'Libera.Chat', host: 'irc.libera.chat', color: 'mint', initial: 'L' },
  { name: 'OFTC', host: 'irc.oftc.net', color: 'peach', initial: 'O' },
  { name: 'Rizon', host: 'irc.rizon.net', color: 'blue', initial: 'R' },
  { name: 'EFnet', host: 'irc.efnet.org', color: 'yellow', initial: 'E' },
]

/** Match the server's endpoint form without accepting URLs or credentials. */
export function normalizeNetworkHost(input: string): string | null {
  // eslint-disable-next-line no-control-regex -- reject control characters from endpoints
  if (!input || input !== input.trim() || /[\s/\\?#@\x00-\x1f]/.test(input)) return null
  if (input.includes(':') || input.startsWith('[') || input.endsWith(']')) {
    const literal = input.startsWith('[') && input.endsWith(']') ? input.slice(1, -1) : input
    if (!literal.includes(':') || literal.includes('[') || literal.includes(']') || literal.includes('%')) return null
    try {
      const hostname = new URL(`http://[${literal}]/`).hostname.slice(1, -1).toLowerCase()
      const mapped = /^::ffff:([\da-f]{1,4}):([\da-f]{1,4})$/.exec(hostname)
      if (mapped) {
        const high = Number.parseInt(mapped[1]!, 16)
        const low = Number.parseInt(mapped[2]!, 16)
        return `${high >> 8}.${high & 255}.${low >> 8}.${low & 255}`
      }
      return hostname
    } catch {
      return null
    }
  }
  const host = input.toLowerCase().replace(/\.$/, '')
  if (!host || host.length > 253 || !/^[a-z0-9.-]+$/.test(host)) return null
  if (host.split('.').some((label) => !label || label.length > 63 || label.startsWith('-') || label.endsWith('-'))) return null
  if (host.includes('.') && /^[\d.]+$/.test(host)) {
    const parts = host.split('.')
    if (parts.length !== 4 || parts.some((part) => Number(part) > 255 || (part.length > 1 && part.startsWith('0')))) return null
  }
  return host
}

/** Public connection hints only. Passwords and account identity stay in the form. */
export function networkPresets(config: unknown, operatorProvided = false): NetworkPreset[] {
  let value = config
  if (typeof value === 'string' && value.trim()) {
    try { value = JSON.parse(value) } catch { return [] }
  } else if (!value) value = defaults
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  return value.slice(0, 20).flatMap((item: unknown): NetworkPreset[] => {
    if (!item || typeof item !== 'object') return []
    const entry = item as Record<string, unknown>
    const name = typeof entry.name === 'string' ? entry.name.trim() : ''
    const host = typeof entry.host === 'string' ? normalizeNetworkHost(entry.host.trim()) : null
    const port = entry.port ?? 6697
    const method = entry.authMethod ?? 'none'
    if (!name || name.length > 64 || !host || typeof port !== 'number' || !Number.isInteger(port) || port < 1 || port > 65535 || !methods.has(method as NetworkPreset['authMethod'])) return []
    const key = `${host.toLowerCase()}:${port}`
    if (seen.has(key)) return []
    seen.add(key)
    // eslint-disable-next-line no-control-regex -- control characters are rejected on purpose
    const autoJoin = Array.isArray(entry.autoJoin) ? entry.autoJoin.filter((channel): channel is string => typeof channel === 'string' && /^[#&][^\s,\x00-\x1f]{1,199}$/.test(channel)).slice(0, operatorProvided ? 100 : 70) : []
    const color = ['mint', 'peach', 'blue', 'yellow'].includes(String(entry.color)) ? entry.color as NetworkPreset['color'] : 'lavender'
    const initial = typeof entry.initial === 'string' && [...entry.initial].length === 1 ? entry.initial : name.slice(0, 1).toUpperCase()
    return [{ name, host, port, tls: entry.tls !== false, authMethod: method as NetworkPreset['authMethod'], autoJoin, color, initial, passwordProvided: entry.passwordProvided === true && method === 'server-password' && entry.tls !== false }]
  })
}
