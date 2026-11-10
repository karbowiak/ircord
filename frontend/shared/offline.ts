import type { Bootstrap, Message, ServerEvent } from './types/chat'
import { messageAttachments, messageMediaLinks } from './utils/link-media'

export const OFFLINE_DAYS = 7
export const OFFLINE_WINDOW = OFFLINE_DAYS * 24 * 60 * 60_000
export const OFFLINE_DB_PREFIX = 'ircord-offline-'
export const OFFLINE_FILE_BUDGET = 256 * 1024 * 1024

export interface OfflineFile {
  url: string
  messageId: string
  conversationId: string
  name: string
  automatic: boolean
}
export interface OfflineStatus {
  phase: 'idle' | 'syncing' | 'ready' | 'partial' | 'unavailable'
  offline: boolean
  messages: number
  files: number
  bytes: number
  since?: string
  updatedAt?: string
  error?: string
}

export function messageFiles(message: Message): OfflineFile[] {
  if (message.deleted) return []
  const files = new Map<string, OfflineFile>()
  const add = (url: string, name: string, automatic: boolean) => {
    if (/^https?:\/\//i.test(url))
      files.set(url, {
        url,
        name,
        automatic,
        messageId: message.id,
        conversationId: message.conversationId,
      })
  }
  for (const attachment of messageAttachments(message))
    add(
      attachment.url,
      attachment.name,
      /^image\//i.test(attachment.contentType ?? '') ||
        /\.(png|jpe?g|webp|avif|gif)$/i.test(attachment.name),
    )
  if (message.gif) add(message.gif.url, message.gif.title || 'GIF', true)
  for (const url of messageMediaLinks(message.text))
    if (!files.has(url) && /\.(png|jpe?g|webp|avif|gif)([?#]|$)/i.test(url)) add(url, 'Image', true)
  return [...files.values()]
}

const upsert = <T extends { id: string }>(items: T[], item: T) => [
  ...items.filter((v) => v.id !== item.id),
  item,
]
/** Archive projection has no UI side effects. Every durable event advances its cursor. */
export type PartyFences = Record<string, { id: string; revision: number; removed: boolean }>
export function archiveState(
  state: Bootstrap,
  event: ServerEvent,
  fences: PartyFences = {},
): Bootstrap {
  if (!('seq' in event) || event.seq <= state.cursor) return state
  if (event.seq !== state.cursor + 1) throw new Error('Offline event gap')
  let next = { ...state, cursor: event.seq }
  switch (event.type) {
    case 'profile.updated':
      next = { ...next, user: event.data, members: upsert(next.members, event.data) }
      break
    case 'member.updated':
      next.members = upsert(next.members, event.data)
      break
    case 'network.updated':
      next.networks = upsert(next.networks, event.data)
      break
    case 'conversation.updated':
      {
        const previous = next.conversations.find((c) => c.id === event.data.id)
        const incoming = event.data
        if (previous?.party && !incoming.party) break
        const fence = fences[incoming.id]
        if (
          fence &&
          (!incoming.party ||
            incoming.party.revision < fence.revision ||
            (fence.removed && incoming.party.revision <= fence.revision))
        )
          break
        if (incoming.party)
          fences[incoming.id] = {
            id: incoming.party.id,
            revision: incoming.party.revision,
            removed: false,
          }
        next.conversations = upsert(
          next.conversations,
          previous?.party && incoming.party && previous.party.revision > incoming.party.revision
            ? { ...incoming, party: previous.party }
            : incoming,
        )
      }
      break
    case 'party.updated':
      if (
        fences[event.data.conversationId] &&
        (event.data.revision < fences[event.data.conversationId]!.revision ||
          (fences[event.data.conversationId]!.removed &&
            event.data.revision <= fences[event.data.conversationId]!.revision))
      )
        break
      fences[event.data.conversationId] = {
        id: event.data.id,
        revision: event.data.revision,
        removed: false,
      }
      next.conversations = next.conversations.map((c) =>
        c.id === event.data.conversationId && (!c.party || c.party.revision < event.data.revision)
          ? { ...c, party: event.data }
          : c,
      )
      break
    case 'party.removed':
      if (
        fences[event.data.conversationId] &&
        event.data.revision < fences[event.data.conversationId]!.revision
      )
        break
      fences[event.data.conversationId] = {
        id: event.data.id,
        revision: event.data.revision,
        removed: true,
      }
      next.conversations = next.conversations.filter(
        (c) =>
          c.id !== event.data.conversationId ||
          (!!c.party && c.party.revision > event.data.revision),
      )
      break
    case 'conversation.counts':
      next.conversations = next.conversations.map((c) =>
        c.id === event.data.id ? { ...c, ...event.data, unreadFrom: event.data.unreadFrom } : c,
      )
      break
    case 'conversation.removed':
      next.conversations = next.conversations.filter((c) => c.id !== event.data.id)
      break
    case 'network.removed':
      next.networks = next.networks.filter((n) => n.id !== event.data.id)
      next.conversations = next.conversations.filter((c) => c.networkId !== event.data.id)
      next.members = next.members.filter((m) => !m.id.startsWith(`${event.data.id}:`))
      break
    case 'invitation.received':
      next.invitations = upsert(next.invitations ?? [], event.data)
      break
    case 'invitation.removed':
      next.invitations = (next.invitations ?? []).filter((i) => i.id !== event.data.id)
      break
  }
  return next
}
