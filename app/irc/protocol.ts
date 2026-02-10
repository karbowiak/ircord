export type IrcMessage = {
  raw: string
  tags: Record<string, string>
  prefix: string | null
  command: string
  params: string[]
  trailing: string
}

export function parseIrcMessage(rawLine: string): IrcMessage {
  let cursor = rawLine.trim()
  const tags: Record<string, string> = {}
  let prefix: string | null = null

  if (cursor.startsWith('@')) {
    const end = cursor.indexOf(' ')
    const tagRaw = end === -1 ? cursor.slice(1) : cursor.slice(1, end)
    for (const pair of tagRaw.split(';')) {
      if (!pair) continue
      const eq = pair.indexOf('=')
      if (eq === -1) {
        tags[pair] = ''
      } else {
        tags[pair.slice(0, eq)] = unescapeTagValue(pair.slice(eq + 1))
      }
    }
    cursor = end === -1 ? '' : cursor.slice(end + 1)
  }

  if (cursor.startsWith(':')) {
    const end = cursor.indexOf(' ')
    prefix = end === -1 ? cursor.slice(1) : cursor.slice(1, end)
    cursor = end === -1 ? '' : cursor.slice(end + 1)
  }

  const tokens: string[] = []
  let trailing = ''
  while (cursor.length > 0) {
    if (cursor.startsWith(':')) {
      trailing = cursor.slice(1)
      break
    }

    const next = cursor.indexOf(' ')
    if (next === -1) {
      tokens.push(cursor)
      break
    }

    tokens.push(cursor.slice(0, next))
    cursor = cursor.slice(next + 1)
    while (cursor.startsWith(' ')) {
      cursor = cursor.slice(1)
    }
  }

  const command = tokens.shift() || ''
  return {
    raw: rawLine,
    tags,
    prefix,
    command,
    params: tokens,
    trailing,
  }
}

export function parseNickname(prefix: string | null): string {
  if (!prefix) return ''
  const bang = prefix.indexOf('!')
  return bang === -1 ? prefix : prefix.slice(0, bang)
}

export function buildTagString(tags: Record<string, string | undefined>): string {
  const pairs = Object.entries(tags)
    .filter(([, value]) => value !== undefined)
    .map(([key, value]) => `${key}=${escapeTagValue(value ?? '')}`)

  if (pairs.length === 0) return ''
  return `@${pairs.join(';')} `
}

export function buildCommand(command: string, params: string[] = [], trailing?: string): string {
  const base = [command, ...params].join(' ').trim()
  if (trailing === undefined) return base
  return `${base} :${trailing}`
}

function unescapeTagValue(value: string): string {
  let output = ''

  for (let i = 0; i < value.length; i += 1) {
    const ch = value[i]
    if (ch !== '\\') {
      output += ch
      continue
    }

    const next = value[i + 1]
    if (next === undefined) {
      output += '\\'
      continue
    }

    i += 1
    switch (next) {
      case ':':
        output += ';'
        break
      case 's':
        output += ' '
        break
      case 'r':
        output += '\r'
        break
      case 'n':
        output += '\n'
        break
      case '\\':
        output += '\\'
        break
      default:
        output += next
        break
    }
  }

  return output
}

function escapeTagValue(value: string): string {
  return value
    .replace(/\\/g, '\\\\')
    .replace(/;/g, '\\:')
    .replace(/ /g, '\\s')
    .replace(/\r/g, '\\r')
    .replace(/\n/g, '\\n')
}
