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

/** Public connection hints only. Passwords and account identity stay in the form. */
export function networkPresets(config: unknown): NetworkPreset[] {
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
    const host = typeof entry.host === 'string' ? entry.host.trim() : ''
    const port = entry.port ?? 6697
    const method = entry.authMethod ?? 'none'
    // eslint-disable-next-line no-control-regex -- control characters are rejected on purpose
    if (!name || name.length > 64 || !host || host.length > 253 || /[\s/\\?#@\x00-\x1f]/.test(host) || typeof port !== 'number' || !Number.isInteger(port) || port < 1 || port > 65535 || !methods.has(method as NetworkPreset['authMethod'])) return []
    const key = `${host.toLowerCase()}:${port}`
    if (seen.has(key)) return []
    seen.add(key)
    // eslint-disable-next-line no-control-regex -- control characters are rejected on purpose
    const autoJoin = Array.isArray(entry.autoJoin) ? entry.autoJoin.filter((channel): channel is string => typeof channel === 'string' && /^[#&][^\s,\x00-\x1f]{1,199}$/.test(channel)).slice(0, 70) : []
    const color = ['mint', 'peach', 'blue', 'yellow'].includes(String(entry.color)) ? entry.color as NetworkPreset['color'] : 'lavender'
    return [{ name, host, port, tls: entry.tls !== false, authMethod: method as NetworkPreset['authMethod'], autoJoin, color, initial: name.slice(0, 1).toUpperCase(), passwordProvided: entry.passwordProvided === true && method === 'server-password' && entry.tls !== false }]
  })
}
