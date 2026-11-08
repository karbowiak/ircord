import type { Conversation, Message, Network } from '../types/chat'
import { channelRights, channelTarget, parsePrefix } from './irc'

export type MessageAction = 'replies' | 'reactions' | 'redact'

export function conversationSendReason(
  conversation: Conversation | undefined,
  network: Network | undefined,
  live = true,
): string | undefined {
  if (!live || network?.status !== 'connected') return 'Reconnect to this network to send messages.'
  if (!conversation || (conversation.kind === 'channel' && !conversation.joined))
    return 'Join this channel to send messages.'
  if (conversation.kind === 'channel' && conversation.modes?.includes('m')) {
    const { modes, prefixes } = parsePrefix(network.isupport)
    const voice = modes.indexOf('v')
    const permitted = [...(conversation.memberModes?.you ?? '')].some((prefix) => {
      const rank = prefixes.indexOf(prefix)
      return voice >= 0 && rank >= 0 && rank <= voice
    })
    if (!permitted)
      return 'This channel is moderated. You need voice or operator privileges to send messages.'
  }
}

/** One decision shared by toolbars, context menus and mutation guards. */
export function messageActionReason(
  message: Message,
  conversation: Conversation | undefined,
  network: Network | undefined,
  action: MessageAction,
  live = true,
): string | undefined {
  if (!live || network?.status !== 'connected')
    return 'Reconnect to this network to use this action.'
  if (!conversation || (conversation.kind === 'channel' && !conversation.joined))
    return 'Join this channel to use this action.'
  if (action !== 'redact') {
    const reason = conversationSendReason(conversation, network, live)
    if (reason) return reason
  }
  if (
    !message.upstreamId ||
    message.upstreamId.startsWith('_') ||
    message.delivery ||
    message.deleted ||
    !['message', 'action', 'notice'].includes(message.kind)
  )
    return 'This message has no usable network message ID.'
  if (action === 'redact' && message.senderId !== 'you')
    return 'You can only delete your own messages.'
  if (!network.features[action]) {
    if (action === 'redact') return 'This network does not support deleting messages'
    const label = { replies: 'Replies', reactions: 'Reactions' }[action]
    return `${label} are unavailable on this network.`
  }
  if (action === 'redact') return redactionAccountReason(message, conversation, network)
}

export const NEEDS_ACCOUNT = 'Deleting needs a logged-in account on this network'
export const OTHER_ACCOUNT =
  'This message wasn’t sent from the account you’re logged into, so the network won’t delete it'

/**
 * Ergo (with history.retention.allow-individual-delete) accepts deleting a message sent from
 * the account you are logged into, or any message in a channel where you are an operator (@ and
 * above). Deletions that would certainly be refused are not offered; the network still has the
 * final say.
 */
function redactionAccountReason(
  message: Message,
  conversation: Conversation,
  network: Network,
): string | undefined {
  if (channelRights('you', conversation, network).op) return undefined
  const account = network.account?.toLowerCase()
  if (!account) return NEEDS_ACCOUNT
  if (message.account?.toLowerCase() !== account) return OTHER_ACCOUNT
}

/** Mirrors the effective byte/line bounds published by Go, after formatting. */
export function messageTextReason(
  text: string,
  kind: 'message' | 'action',
  conversation: Conversation,
  network: Network,
): string | undefined {
  const wire = kind === 'action' ? `\x01ACTION ${text}\x01` : text
  const bytes = new TextEncoder().encode(wire).length
  if ([...text].length > 4000 || new TextEncoder().encode(text).length > 4096)
    return 'Use at most 4000 characters and 4096 UTF-8 bytes per message.'
  const target =
    conversation.kind === 'channel'
      ? channelTarget(conversation)
      : (conversation.target ?? conversation.name)
  const budget =
    network.features.messageLineBytes - new TextEncoder().encode(`PRIVMSG ${target} :`).length
  if (budget < 4) return 'This network has not supplied a usable message line limit.'
  if (bytes <= budget && !wire.includes('\n')) return
  const limits = network.features.multiline
  if (!limits)
    return `This network does not support multiline messages. Use one line of at most ${budget} UTF-8 bytes.`
  if (bytes > limits.maxBytes)
    return `This network allows at most ${limits.maxBytes} UTF-8 bytes per multiline message.`
  let lines = 0
  for (const line of wire.split('\n')) {
    let used = 0
    lines++
    for (const char of line) {
      const size = new TextEncoder().encode(char).length
      if (used + size > budget) {
        lines++
        used = 0
      }
      used += size
    }
  }
  if (lines > limits.maxLines)
    return `This network allows at most ${limits.maxLines} wire lines per multiline message.`
}
