// irc:// and ircs:// links (draft-butcher-irc-url): irc://host[:port]/target[,flags][?key].
// Targets are channels (with or without “#”), or nicks with the “isnick” flag.

export interface IrcLink {
  host: string
  port: number
  tls: boolean
  /** Channel name including its prefix, e.g. “#rust”. */
  channel?: string
  key?: string
  nick?: string
}

/** Parses an irc(s):// URL; null when it isn't one or is malformed. */
export function parseIrcUrl(value: string): IrcLink | null {
  const match = /^(ircs?):\/\/([^/:?#\s]+)(?::(\d{1,5}))?(?:\/([^?\s]*))?(?:\?(\S*))?$/i.exec(
    value.trim(),
  )
  if (!match) return null
  const tls = match[1]!.toLowerCase() === 'ircs'
  const host = match[2]!.toLowerCase()
  if (!/^[a-z0-9.-]{1,253}$/.test(host)) return null
  const port = match[3] ? Number(match[3]) : tls ? 6697 : 6667
  if (port < 1 || port > 65535) return null
  const link: IrcLink = { host, port, tls }
  const target = decodeURIComponent(match[4] ?? '')
  if (target) {
    const [name = '', ...flags] = target.split(',')
    if (flags.includes('isnick')) link.nick = name
    else if (name) link.channel = /^[#&!+]/.test(name) ? name : `#${name}`
  }
  if (match[5] && link.channel) link.key = decodeURIComponent(match[5])
  return link
}

/** Network names derived from a host: irc.libera.chat → “libera.chat”. */
export const networkNameFromHost = (host: string) => host.replace(/^(irc|chat)\./, '')
