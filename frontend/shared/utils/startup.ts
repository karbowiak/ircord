import type { ClientPreferences } from '../preferences'
import type { Bootstrap, Conversation } from '../types/chat'
import { channelTarget } from './irc'

/** A left or kicked channel must not be reopened by a saved startup choice. */
export function startupChannels(data: Bootstrap): Conversation[] {
  const networks = new Set(data.networks.map((network) => network.id))
  return data.conversations
    .filter(
      (conversation) =>
        conversation.kind === 'channel' &&
        conversation.joined !== false &&
        !conversation.kicked &&
        networks.has(conversation.networkId),
    )
    .sort((a, b) => a.networkId.localeCompare(b.networkId) || a.id.localeCompare(b.id))
}

/** Choose from the current snapshot only, with stable fallbacks for removed conversations. */
export function startupConversation(
  data: Bootstrap,
  startup: ClientPreferences['startup'],
  lastId: string,
): string {
  const networkIds = new Set(data.networks.map((network) => network.id))
  const channels = startupChannels(data)
  const channelIds = new Set(channels.map((channel) => channel.id))
  const valid = data.conversations
    .filter(
      (conversation) =>
        networkIds.has(conversation.networkId) &&
        (conversation.kind === 'direct' || channelIds.has(conversation.id)),
    )
    .sort((a, b) => a.networkId.localeCompare(b.networkId) || a.id.localeCompare(b.id))
  const general = channels.find((channel) => channelTarget(channel).toLowerCase() === '#general')
  const preferred =
    startup.mode === 'channel'
      ? startup.channelId
        ? channels.find((channel) => channel.id === startup.channelId)
        : general
      : valid.find((conversation) => conversation.id === lastId)
  const last = valid.find((conversation) => conversation.id === lastId)
  return (
    preferred?.id ??
    last?.id ??
    general?.id ??
    valid[0]?.id ??
    (data.networks.length ? `server:${data.networks.map((network) => network.id).sort()[0]}` : '')
  )
}
