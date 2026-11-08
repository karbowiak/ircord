import type { Conversation } from '../../shared/types/chat'
import type { AccessEntry, ChannelRegistration, ServicesRole } from '../../shared/types/services'
import { channelRights } from '../../shared/utils/irc'
import { getConversation, getNetwork, emit, type DemoSession } from './demo-store'

/**
 * Demo ChanServ, following the Ergo behaviour recorded in #9: registration, founder info,
 * persistent access (AMODE) and transfer. OFTC stands in for a network without supported
 * services. The observed account belongs to the selected network.
 */
interface Registration {
  founder: string
  registeredAt: string
  access: AccessEntry[]
}
const registrations = new WeakMap<DemoSession, Record<string, Registration | null>>()

const fail = (statusCode: number, statusMessage: string) =>
  createError({ statusCode, statusMessage })

function channel(session: DemoSession, id: string) {
  let c: Conversation
  try {
    c = getConversation(session, id)
  } catch {
    throw fail(404, 'Conversation not found.')
  }
  const network = getNetwork(session, c.networkId)
  if (c.kind !== 'channel') throw fail(400, 'Only channels can be registered.')
  if (network.id === 'oftc')
    throw fail(501, `${network.name} has no ChanServ that IRCord supports.`)
  if (network.status !== 'connected') throw fail(409, `${network.name} is disconnected.`)
  const all = (registrations.get(session) ?? seed(session))!
  return { c, network, account: network.account ?? '', all }
}

function seed(session: DemoSession) {
  const at = new Date(Date.now() - 400 * 86_400_000).toISOString()
  const you = getNetwork(session, 'libera').account ?? ''
  const all: Record<string, Registration | null> = {
    general: {
      founder: you,
      registeredAt: at,
      access: [
        { account: you, mode: 'q' },
        { account: 'mika', mode: 'o' },
        { account: 'sage', mode: 'v' },
      ],
    },
    development: { founder: 'mika', registeredAt: at, access: [{ account: 'mika', mode: 'q' }] },
  }
  registrations.set(session, all)
  return all
}

const known = (session: DemoSession, account: string) =>
  session.bootstrap.members.some((m) => m.nick.toLowerCase() === account.toLowerCase())

function registered(all: Record<string, Registration | null>, id: string) {
  const r = all[id]
  if (!r) throw fail(404, 'This channel isn’t registered.')
  return r
}
function founderOnly(r: Registration, account: string) {
  if (r.founder.toLowerCase() !== account.toLowerCase()) throw fail(403, 'Insufficient privileges.')
}

export function chanservInfo(session: DemoSession, id: string): ChannelRegistration {
  const { all, c, account } = channel(session, id)
  const r = all[c.id]
  return r
    ? { registered: true, founder: r.founder, registeredAt: r.registeredAt }
    : { registered: false }
}

export function chanservRegister(session: DemoSession, id: string): ChannelRegistration {
  const { all, c, network, account } = channel(session, id)
  if (all[c.id]) throw fail(409, 'This channel is already registered.')
  if (!channelRights('you', c, network).op)
    throw fail(403, 'You need operator status (@) in the channel to register it.')
  all[c.id] = {
    founder: account,
    registeredAt: new Date().toISOString(),
    access: [{ account, mode: 'q' }],
  }
  emit(session, { type: 'channel-services.updated', data: { conversationId: id } })
  return chanservInfo(session, id)
}

export function chanservUnregister(session: DemoSession, id: string) {
  const { all, c, account } = channel(session, id)
  founderOnly(registered(all, c.id), account)
  all[c.id] = null
  emit(session, { type: 'channel-services.updated', data: { conversationId: id } })
}

export function chanservAccess(session: DemoSession, id: string) {
  const { all, c, account } = channel(session, id)
  const r = registered(all, c.id)
  if (!r.access.some((e) => e.account === account && 'qao'.includes(e.mode)))
    throw fail(403, 'Insufficient privileges.')
  return { items: r.access }
}

export function chanservSetAccess(
  session: DemoSession,
  id: string,
  target: string,
  mode: ServicesRole,
) {
  const { all, c, account } = channel(session, id)
  const r = registered(all, c.id)
  founderOnly(r, account)
  if (!'aohv'.includes(mode) || mode.length !== 1) throw fail(400, 'Unknown role.')
  if (!known(session, target)) throw fail(422, 'Account does not exist.')
  const entry = r.access.find((e) => e.account.toLowerCase() === target.toLowerCase())
  if (entry) entry.mode = mode
  else r.access.push({ account: target, mode })
  emit(session, { type: 'channel-services.updated', data: { conversationId: id } })
  return { items: r.access }
}

export function chanservRemoveAccess(session: DemoSession, id: string, target: string) {
  const { all, c, account } = channel(session, id)
  const r = registered(all, c.id)
  founderOnly(r, account)
  if (target === r.founder) throw fail(400, 'Transfer the channel to change its founder.')
  r.access = r.access.filter((e) => e.account.toLowerCase() !== target.toLowerCase())
  emit(session, { type: 'channel-services.updated', data: { conversationId: id } })
  return { items: r.access }
}

export function chanservTransfer(session: DemoSession, id: string, target: string) {
  const { all, c, account } = channel(session, id)
  founderOnly(registered(all, c.id), account)
  if (!known(session, target)) throw fail(422, 'Account does not exist.')
  return { status: 'pending' as const }
}
