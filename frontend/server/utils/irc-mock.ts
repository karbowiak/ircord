import { nicknameError, nicknameLimit } from '../../shared/utils/nickname'
import { randomUUID } from 'node:crypto'
import { wireConversation } from '../../shared/utils/conversation'
import type {
  Attachment,
  ChannelListEntry,
  ChannelListing,
  Conversation,
  MembershipEvent,
  Network,
  NetworkAuth,
  NetworkInput,
  ServerLine,
  ServerLogPage,
  Upload,
} from '../../shared/types/chat'
import { channelRights, parsePrefix, validRawLine } from '../../shared/utils/irc'
import { parseChanmodes } from '../../shared/utils/channel-modes'
import {
  addConversation,
  appendMessage,
  emit,
  getNetwork,
  limits,
  type DemoSession,
  type StoredUpload,
} from './demo-store'

/*
 * Simulated IRC behaviour for the demo: the status window, raw commands, channel modes and
 * membership events. It stands in for the Go service (BACKEND-REQUESTS.md §2–§8) and is not an
 * IRC implementation.
 */

// ---- Server log -----------------------------------------------------------------------------

export function serverLine(
  session: DemoSession,
  networkId: string,
  line: Omit<ServerLine, 'id' | 'networkId' | 'time'>,
  announce = true,
) {
  const entry: ServerLine = { id: randomUUID(), networkId, time: new Date().toISOString(), ...line }
  const log = (session.serverLogs[networkId] ??= [])
  log.push(entry)
  if (log.length > limits.serverLines) log.splice(0, log.length - limits.serverLines)
  if (announce) emit(session, { type: 'server.line', data: entry })
  return entry
}
/** A numeric reply. `display` is the human rendering when it should differ from the trailing text. */
const numeric = (
  session: DemoSession,
  network: Network,
  code: string,
  text: string,
  params: string[] = [],
  display = params.length ? `${params.join(' ')}: ${text}` : text,
) =>
  serverLine(session, network.id, {
    kind: /^[45]/.test(code) ? 'error' : /^37[256]$/.test(code) ? 'motd' : 'numeric',
    command: code,
    source: network.host,
    params: [network.nick, ...params, text],
    text: display,
  })

export function seedServerLog(session: DemoSession, network: Network) {
  const quiet = (line: Omit<ServerLine, 'id' | 'networkId' | 'time'>) =>
    serverLine(session, network.id, line, false)
  const from = network.host
  quiet({
    kind: 'notice',
    command: 'NOTICE',
    source: from,
    params: ['*'],
    text: '*** Looking up your hostname...',
  })
  quiet({
    kind: 'notice',
    command: 'NOTICE',
    source: from,
    params: ['*'],
    text: '*** Found your hostname',
  })
  const isupport = Object.entries(network.isupport).map(([k, v]) => `${k}=${v}`)
  const lines: [string, string, string[]?][] = [
    ['001', `Welcome to the ${network.name} Internet Relay Chat Network ${network.nick}`],
    ['002', `Your host is ${from}, running version demo-1.0`],
    ['003', 'This server was created at the start of this demo session'],
    ['004', `${from} demo-1.0 DOQRSZaghilopsuwz CFILMPQSbcefgijklmnopqrstuvz`],
    ['005', 'are supported by this server', isupport],
    ['251', 'There are 412 users and 31337 invisible on 24 servers'],
    ['375', `- ${from} Message of the Day -`],
    ['372', `- Welcome to \x02${network.name}\x02. This is a demo server run by IRCord.`],
    ['372', '- Be excellent to each other. \x0303Have fun.\x03'],
    ['372', '- Nothing typed here reaches a real IRC network.'],
    ['376', 'End of /MOTD command.'],
  ]
  for (const [code, text, params] of lines)
    quiet({
      kind: /^37[256]$/.test(code) ? 'motd' : 'numeric',
      command: code,
      source: from,
      params: [network.nick, ...(params ?? []), text],
      text: params ? `${params.join(' ')} :${text}` : text,
    })
  if (network.auth.method !== 'none')
    quiet({
      kind: 'notice',
      command: 'NOTICE',
      source: 'NickServ!NickServ@services.',
      params: [network.nick],
      text: `You are now identified for \x02${network.nick}\x02.`,
    })
}

export function serverLog(
  session: DemoSession,
  networkId: string,
  before?: string,
  limit = 100,
): ServerLogPage {
  getNetwork(session, networkId)
  const log = session.serverLogs[networkId] ?? []
  const end = before ? log.findIndex((l) => l.id === before) : log.length
  if (end < 0) throw new Error('This log cursor has expired. Reopen the server window.')
  const start = Math.max(0, end - Math.min(200, Math.max(1, limit)))
  return { items: log.slice(start, end), nextCursor: start > 0 ? log[start]!.id : null }
}

// ---- Membership events ----------------------------------------------------------------------

export function membershipEvent(session: DemoSession, c: Conversation, event: MembershipEvent) {
  const nick = (id?: string) => session.bootstrap.members.find((m) => m.id === id)?.nick ?? id ?? ''
  const summary = {
    join: `${nick(event.actorId)} joined`,
    part: `${nick(event.actorId)} left${event.text ? ` (${event.text})` : ''}`,
    quit: `${nick(event.actorId)} quit${event.text ? ` (${event.text})` : ''}`,
    nick: `${nick(event.actorId)} is now known as ${event.text}`,
    kick: `${nick(event.actorId)} kicked ${nick(event.targetId)}${event.text ? ` (${event.text})` : ''}`,
    mode: `${nick(event.actorId)} set mode ${event.text}`,
    topic: `${nick(event.actorId)} changed the topic to: ${event.text}`,
    invite: `${nick(event.actorId)} invited ${nick(event.targetId)} to ${event.text}`,
    registered: `${nick(event.actorId)} created this channel, so it’s registered to their account`,
    'registration-failed': `${nick(event.actorId)} created this channel, but it couldn’t be registered: ${event.text}`,
  }[event.type]
  appendMessage(session, {
    id: randomUUID(),
    conversationId: c.id,
    senderId: event.actorId,
    text: summary,
    createdAt: new Date().toISOString(),
    kind: 'event',
    event,
    reactions: [],
  })
}

export function removeConversation(session: DemoSession, id: string) {
  const c = session.bootstrap.conversations.find((x) => x.id === id)
  if (!c) throw new Error('Conversation not found.')
  session.bootstrap.conversations = session.bootstrap.conversations.filter((x) => x.id !== id)
  delete session.messages[id]
  emit(session, { type: 'conversation.removed', data: { id } })
}

// ---- Channel lists (bans, exceptions, invite exceptions, quiets) ----------------------------

const listNumerics: Record<string, [string, string]> = {
  b: ['367', '368'],
  e: ['348', '349'],
  I: ['346', '347'],
  q: ['728', '729'],
}
export function channelList(
  session: DemoSession,
  c: Conversation,
  mode: string,
): ChannelListEntry[] {
  const lists = (session.channelLists[c.id] ??= {})
  return (lists[mode] ??= [])
}

// ---- Raw commands ---------------------------------------------------------------------------

const managed = new Set(['QUIT', 'USER', 'PASS', 'CAP', 'AUTHENTICATE'])

function parseLine(line: string) {
  const index = line.indexOf(' :')
  const head = index >= 0 ? line.slice(0, index) : line
  const words = head.trim().split(/\s+/)
  const params = words.slice(1)
  if (index >= 0) params.push(line.slice(index + 2))
  return { command: words[0]!.toUpperCase(), params }
}

export function sendRaw(session: DemoSession, networkId: string, line: unknown) {
  if (!validRawLine(line)) throw new Error('Commands must be a single line of at most 510 bytes.')
  const network = getNetwork(session, networkId)
  if (network.status !== 'connected') throw new Error(`${network.name} is disconnected.`)
  const { command, params } = parseLine(line)
  if (managed.has(command)) throw new Error(`${command} is handled by IRCord.`)
  serverLine(session, network.id, { kind: 'outgoing', command, params, text: line })

  const user = session.bootstrap.user
  const memberByNick = (nick = '') =>
    session.bootstrap.members.find((m) => m.nick.toLowerCase() === nick.toLowerCase())
  const channel = (name = '') => {
    const c = session.bootstrap.conversations.find(
      (x) =>
        x.networkId === network.id &&
        x.kind === 'channel' &&
        x.name.toLowerCase() === name.replace(/^#/, '').toLowerCase(),
    )
    if (!c) {
      numeric(session, network, '403', 'No such channel', [name])
      throw new Error(`You’re not in ${name || 'that channel'}.`)
    }
    return c
  }
  const refuse = (c: Conversation, why: string) => {
    numeric(session, network, '482', why, [`#${c.name}`])
    throw new Error(`${why} in #${c.name}.`)
  }
  const changed = (c: Conversation) =>
    emit(session, { type: 'conversation.updated', data: wireConversation(c) })

  switch (command) {
    case 'NICK': {
      const nick = params[0] ?? ''
      const problem =
        params.length !== 1
          ? 'Nicknames cannot contain spaces.'
          : nicknameError(nick, nicknameLimit(network.isupport))
      if (problem) {
        numeric(session, network, '432', 'Erroneous nickname', [nick])
        throw new Error(problem)
      }
      const taken = memberByNick(nick)
      if (taken && taken.id !== 'you') {
        numeric(session, network, '433', 'Nickname is already in use', [nick])
        throw new Error(`${nick} is already in use.`)
      }
      const old = user.nick
      network.nick = nick
      emit(session, { type: 'network.updated', data: network })
      user.nick = nick
      emit(session, { type: 'profile.updated', data: user })
      for (const c of session.bootstrap.conversations)
        if (c.networkId === network.id && c.kind === 'channel' && c.memberIds.includes('you'))
          membershipEvent(session, c, { type: 'nick', actorId: 'you', text: nick })
      serverLine(session, network.id, {
        kind: 'raw',
        command: 'NICK',
        source: old,
        params: [nick],
        text: `${old} is now known as ${nick}`,
      })
      return
    }
    case 'SETNAME': {
      if (!network.features.setname) {
        numeric(session, network, '421', 'Unknown command', ['SETNAME'])
        throw new Error(`${network.name} can’t change your real name while connected.`)
      }
      network.realname = (params[0] ?? '').slice(0, 100)
      emit(session, { type: 'network.updated', data: network })
      return void serverLine(session, network.id, {
        kind: 'raw',
        command: 'SETNAME',
        source: network.nick,
        params,
        text: `Your real name is now ${network.realname}`,
      })
    }
    case 'AWAY': {
      user.status = params[0] ? 'away' : 'online'
      user.awayMessage = params[0] || undefined
      emit(session, { type: 'profile.updated', data: user })
      numeric(
        session,
        network,
        params[0] ? '306' : '305',
        params[0] ? 'You have been marked as being away' : 'You are no longer marked as being away',
      )
      return
    }
    case 'TOPIC': {
      const c = channel(params[0])
      if (params.length < 2) return void numeric(session, network, '332', c.topic, [`#${c.name}`])
      if (!channelRights('you', c, network).topic) refuse(c, 'You’re not a channel operator')
      c.topic = params[1]!.slice(0, Number(network.isupport.TOPICLEN ?? 390))
      c.topicSetBy = network.nick
      c.topicSetAt = new Date().toISOString()
      changed(c)
      membershipEvent(session, c, { type: 'topic', actorId: 'you', text: c.topic })
      return
    }
    case 'MODE': {
      if (!params[0]?.startsWith('#')) return void numeric(session, network, '221', '+i', [])
      const c = channel(params[0])
      if (params.length < 2)
        return void numeric(session, network, '324', `+${c.modes ?? ''}`, [`#${c.name}`])
      const rights = channelRights('you', c, network)
      const { modes: prefixModes, prefixes } = parsePrefix(network.isupport)
      const groups = parseChanmodes(network.isupport)
      const args = params.slice(2)
      let adding = true
      const applied: string[] = []
      const appliedArgs: string[] = []
      for (const char of params[1]!) {
        if (char === '+' || char === '-') {
          adding = char === '+'
          continue
        }
        const prefixIndex = prefixModes.indexOf(char)
        if (prefixIndex >= 0) {
          const target = memberByNick(args.shift())
          if (!target || !c.memberIds.includes(target.id))
            throw new Error('They aren’t in this channel.')
          const needsOp = 'qao'.includes(char)
          if (needsOp ? !rights.op : !rights.voice) refuse(c, 'You’re not a channel operator')
          const prefix = prefixes[prefixIndex]!
          const current = c.memberModes?.[target.id] ?? ''
          const next = adding
            ? [...new Set(current + prefix)]
                .sort((a, b) => prefixes.indexOf(a) - prefixes.indexOf(b))
                .join('')
            : current.replace(prefix, '')
          c.memberModes = { ...c.memberModes, [target.id]: next }
          if (!next) delete c.memberModes[target.id]
          appliedArgs.push(target.nick)
        } else if (groups.list.includes(char)) {
          if (!rights.ban) refuse(c, 'You’re not a channel operator')
          const mask = args.shift()
          if (!mask) {
            // No mask: list the entries, as servers do (367/368 for bans and their siblings).
            const [entry, end] = listNumerics[char] ?? ['367', '368']
            for (const item of channelList(session, c, char))
              numeric(
                session,
                network,
                entry,
                item.setBy ?? '',
                [`#${c.name}`, item.mask],
                `#${c.name} ${item.mask}`,
              )
            numeric(session, network, end, 'End of list', [`#${c.name}`])
            continue
          }
          const list = channelList(session, c, char)
          const index = list.findIndex((x) => x.mask.toLowerCase() === mask.toLowerCase())
          if (adding && index < 0)
            list.push({ mask, setBy: network.nick, setAt: new Date().toISOString() })
          if (!adding && index >= 0) list.splice(index, 1)
          appliedArgs.push(mask)
        } else if (groups.always.includes(char) || groups.whenSet.includes(char)) {
          if (!rights.op) refuse(c, 'You’re not a channel operator')
          // Type B takes a parameter both ways; type C only when set.
          const value = adding || groups.always.includes(char) ? args.shift() : undefined
          if (adding && !value) throw new Error(`+${char} needs a value.`)
          if (char === 'l' && adding && !/^\d{1,6}$/.test(value!))
            throw new Error('The limit must be a number.')
          c.modeParams = { ...c.modeParams }
          if (adding) c.modeParams[char] = value!
          else delete c.modeParams[char]
          const set = new Set(c.modes ?? '')
          if (adding) set.add(char)
          else set.delete(char)
          c.modes = [...set].sort().join('')
          if (adding) appliedArgs.push(char === 'k' ? '*' : value!)
        } else if (groups.flags.includes(char)) {
          if (!rights.op) refuse(c, 'You’re not a channel operator')
          const set = new Set(c.modes ?? '')
          if (adding) set.add(char)
          else set.delete(char)
          c.modes = [...set].sort().join('')
        } else {
          numeric(session, network, '472', 'is an unknown mode char to me', [char])
          throw new Error(`Unknown channel mode “${char}”.`)
        }
        applied.push(`${adding ? '+' : '-'}${char}`)
      }
      if (!applied.length) return
      changed(c)
      membershipEvent(session, c, {
        type: 'mode',
        actorId: 'you',
        text: [
          applied
            .join('')
            .replace(/([+-])(\w)(?=\1)/g, '$1$2')
            .replace(/(?<=[a-z])\+/g, ''),
          ...appliedArgs,
        ].join(' '),
      })
      return
    }
    case 'KICK': {
      const c = channel(params[0])
      if (!channelRights('you', c, network).kick) refuse(c, 'You’re not a channel operator')
      const target = memberByNick(params[1])
      if (!target || !c.memberIds.includes(target.id))
        throw new Error('They aren’t in this channel.')
      c.memberIds = c.memberIds.filter((id) => id !== target.id)
      if (c.memberModes) delete c.memberModes[target.id]
      changed(c)
      membershipEvent(session, c, {
        type: 'kick',
        actorId: 'you',
        targetId: target.id,
        text: params[2],
      })
      return
    }
    case 'INVITE': {
      const target = memberByNick(params[0])
      if (!target) throw new Error(`No one called ${params[0]} is online.`)
      return void numeric(session, network, '341', `Inviting ${target.nick} to ${params[1]}`, [
        target.nick,
        params[1] ?? '',
      ])
    }
    case 'WHOIS': {
      const target = memberByNick(params.at(-1))
      if (!target) {
        numeric(session, network, '401', 'No such nick/channel', [params.at(-1) ?? ''])
        throw new Error(`No one called ${params.at(-1)} is online.`)
      }
      const channels = session.bootstrap.conversations
        .filter(
          (c) =>
            c.networkId === network.id && c.kind === 'channel' && c.memberIds.includes(target.id),
        )
        .map((c) => `${c.memberModes?.[target.id]?.[0] ?? ''}#${c.name}`)
      const n = target.nick
      numeric(
        session,
        network,
        '311',
        target.displayName,
        [n, target.account, 'demo.ircord', '*'],
        `\x02${n}\x02 (${target.account}@demo.ircord): ${target.displayName}`,
      )
      numeric(session, network, '319', channels.join(' '), [n], `${n} is on ${channels.join(' ')}`)
      numeric(
        session,
        network,
        '312',
        'IRCord demo server',
        [n, network.host],
        `${n} is connected via ${network.host} (IRCord demo server)`,
      )
      if (target.status === 'away') numeric(session, network, '301', 'Away', [n], `${n} is away`)
      numeric(
        session,
        network,
        '330',
        'is logged in as',
        [n, target.account],
        `${n} is logged in as ${target.account}`,
      )
      return void numeric(
        session,
        network,
        '318',
        'End of /WHOIS list.',
        [n],
        `${n}: End of /WHOIS list.`,
      )
    }
    case 'JOIN': {
      for (const name of (params[0] ?? '').split(','))
        addConversation(session, { networkId: network.id, name })
      return
    }
    case 'PART': {
      removeConversation(session, channel(params[0]).id)
      return
    }
    case 'PING':
      return void serverLine(session, network.id, {
        kind: 'raw',
        command: 'PONG',
        source: network.host,
        params,
        text: `PONG ${params.join(' ')}`,
      })
    case 'LIST': {
      numeric(session, network, '321', 'Users  Name', ['Channel'])
      for (const item of listChannels(session, network.id))
        numeric(session, network, '322', item.topic, [item.name, String(item.users)])
      return void numeric(session, network, '323', 'End of /LIST')
    }
    case 'MOTD':
      seedServerLog(session, { ...network, auth: { method: 'none', passwordSet: false } })
      return void numeric(session, network, '376', 'End of /MOTD command.')
    case 'PRIVMSG':
    case 'NOTICE':
      return void serverLine(session, network.id, {
        kind: 'raw',
        command,
        source: network.nick,
        params,
        text: `→ ${params[0]}: ${params.at(-1)}`,
      })
    default:
      numeric(session, network, '421', 'Unknown command', [command])
      throw new Error(`The server doesn’t know “${command}”.`)
  }
}

// ---- Network settings -----------------------------------------------------------------------

/** Validates client input into stored network fields. Passwords only set `passwordSet`. */
export function networkSettings(input: NetworkInput, existing?: Network) {
  const name = typeof input.name === 'string' ? input.name.trim() : ''
  const port = Number(input.port)
  if (
    !name ||
    name.length > 40 ||
    !/^[a-zA-Z0-9.-]{1,253}$/.test(input.host ?? '') ||
    !Number.isInteger(port) ||
    port < 1 ||
    port > 65535
  )
    throw new Error('Enter a network name, hostname and valid port.')
  const nick = typeof input.nick === 'string' ? input.nick : ''
  const problem = nicknameError(nick, nicknameLimit(existing?.isupport))
  if (problem) throw new Error(problem)
  if (input.altNick && nicknameError(input.altNick, nicknameLimit(existing?.isupport)))
    throw new Error(
      `Alternate nickname: ${nicknameError(input.altNick, nicknameLimit(existing?.isupport))}`,
    )
  const text = (value: unknown, max: number) =>
    typeof value === 'string' ? value.trim().slice(0, max) : undefined
  const previous = existing?.auth as { passwordSet?: boolean } | undefined
  const passwordSet = (password: unknown) =>
    typeof password === 'string'
      ? password.length > 0
      : previous?.passwordSet === true && existing?.auth.method === input.auth?.method
  let auth: NetworkAuth
  switch (input.auth?.method) {
    case 'sasl-plain':
      if (!text(input.auth.account, 64)) throw new Error('Enter the account name for SASL.')
      auth = {
        method: 'sasl-plain',
        account: text(input.auth.account, 64)!,
        passwordSet: passwordSet(input.auth.password),
      }
      break
    case 'sasl-external':
      auth = { method: 'sasl-external', passwordSet: false }
      break
    case 'server-password':
    case 'nickserv':
      auth = { method: input.auth.method, passwordSet: passwordSet(input.auth.password) }
      break
    default:
      auth = { method: 'none', passwordSet: false }
  }
  const autoJoin = (Array.isArray(input.autoJoin) ? input.autoJoin : [])
    .map((entry) => String(entry).trim())
    .filter(Boolean)
    .slice(0, 50)
  if (autoJoin.some((entry) => !/^[#&][^\s,]{1,49}( \S{1,23})?$/.test(entry)))
    throw new Error('Auto-join entries look like “#channel” or “#channel key”.')
  const perform = (Array.isArray(input.perform) ? input.perform : [])
    .map((l) => String(l).trim())
    .filter(Boolean)
    .slice(0, 20)
  if (perform.some((l) => !validRawLine(l)))
    throw new Error('Each on-connect command must be a single IRC line.')
  return {
    name,
    host: input.host,
    port,
    tls: input.tls !== false,
    nick,
    altNick: text(input.altNick, 32) || undefined,
    username: text(input.username, 32) || undefined,
    realname: text(input.realname, 100) || undefined,
    auth,
    autoJoin,
    perform,
  }
}
export function storeSecret(session: DemoSession, networkId: string, input: NetworkInput) {
  const password = (input.auth as { password?: unknown } | undefined)?.password
  if (typeof password === 'string') {
    if (password) session.secrets[networkId] = password
    else delete session.secrets[networkId]
  }
}

// ---- Channel list ---------------------------------------------------------------------------

const directory: ChannelListing[] = [
  {
    name: '#linux',
    users: 1843,
    topic: 'Linux support and discussion. Read the topic before asking.',
  },
  { name: '#python', users: 1502, topic: 'Python help: paste code at a pastebin, not in channel.' },
  { name: '#rust', users: 812, topic: 'The Rust programming language' },
  { name: '#go-nuts', users: 640, topic: 'Go: simple, reliable, efficient' },
  { name: '#ircv3', users: 214, topic: 'IRCv3 working group: specs at ircv3.net' },
  { name: '#photography', users: 188, topic: 'Share your shots, gear talk welcome' },
  { name: '#coffee', users: 77, topic: 'Brewing, beans and bad decisions' },
  { name: '#weekend-plans', users: 12, topic: 'What are you up to?' },
]
export function listChannels(
  session: DemoSession,
  networkId: string,
  q = '',
  limit = 50,
): ChannelListing[] {
  getNetwork(session, networkId)
  const mine = session.bootstrap.conversations
    .filter((c) => c.networkId === networkId && c.kind === 'channel')
    .map((c) => ({ name: `#${c.name}`, users: c.memberIds.length, topic: c.topic }))
  const all = [...mine, ...directory.filter((d) => !mine.some((m) => m.name === d.name))]
  const needle = q.toLowerCase().replace(/^#/, '')
  return all
    .filter(
      (c) =>
        !needle || c.name.toLowerCase().includes(needle) || c.topic.toLowerCase().includes(needle),
    )
    .sort((a, b) => b.users - a.users)
    .slice(0, Math.min(100, limit))
}

// ---- Uploads --------------------------------------------------------------------------------

const allowed = (type: string) =>
  /^(image\/(png|jpeg|gif|webp|avif)|video\/mp4|text\/plain|application\/pdf)$/.test(type)
export function storeUpload(
  session: DemoSession,
  file: { name: string; type: string; data: Uint8Array; width?: number; height?: number },
): Upload {
  if (!allowed(file.type)) throw new Error('This file type can’t be uploaded.')
  if (file.data.byteLength > limits.uploadBytes) throw new Error('Files can be up to 8 MB.')
  const id = randomUUID()
  const dimension = (value?: number) =>
    Number.isInteger(value) && value! > 0 && value! <= 16384 ? value : undefined
  const upload: StoredUpload = {
    id,
    url: `/api/v1/uploads/${id}`,
    name: file.name.replace(/[^\w.\- ]+/g, '_').slice(0, 120) || 'file',
    size: file.data.byteLength,
    contentType: file.type,
    width: dimension(file.width),
    height: dimension(file.height),
    bytes: file.data,
  }
  session.uploads.set(id, upload)
  while (session.uploads.size > limits.uploads)
    session.uploads.delete(session.uploads.keys().next().value!)
  const { bytes: _bytes, ...view } = upload
  return view
}
export function attachmentFor(upload: Upload): Attachment {
  return {
    name: upload.name,
    url: upload.url,
    width: upload.width ?? 0,
    height: upload.height ?? 0,
    alt: upload.name,
    contentType: upload.contentType,
    size: upload.size,
  }
}
