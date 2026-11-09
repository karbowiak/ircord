import type { Conversation, Message, Network } from '../types/chat'
import { channelRights, channelTarget, parsePrefix } from './irc'

export type MessageAction = 'replies' | 'reactions' | 'redact' | 'edit'
export type MessageReasonTranslator = (
  key: string,
  values?: Record<string, string | number>,
) => string

const englishReason: MessageReasonTranslator = (key, values) => {
  const messages: Record<string, string> = {
    'collectionsUi.features.send.reconnect': 'Reconnect to this network to send messages.',
    'collectionsUi.features.send.join': 'Join this channel to send messages.',
    'collectionsUi.features.send.moderated':
      'This channel is moderated. You need voice or operator privileges to send messages.',
    'collectionsUi.features.action.reconnect': 'Reconnect to this network to use this action.',
    'collectionsUi.features.action.join': 'Join this channel to use this action.',
    'collectionsUi.features.action.invalidId': 'This message has no usable network message ID.',
    'collectionsUi.features.redact.ownOnly': 'You can only delete your own messages.',
    'collectionsUi.features.edit.ownOnly': 'You can only edit your own messages.',
    'collectionsUi.features.edit.notice': 'Notices cannot be edited.',
    'collectionsUi.features.edit.media': 'GIFs and uploads cannot be edited.',
    'collectionsUi.features.replies.unavailable': 'Replies are unavailable on this network.',
    'collectionsUi.features.reactions.unavailable': 'Reactions are unavailable on this network.',
    'collectionsUi.features.redact.unsupported': 'This network does not support deleting messages',
    'collectionsUi.features.edit.unsupported': EDIT_UNSUPPORTED,
    'collectionsUi.features.edit.accountRequired': EDIT_NEEDS_ACCOUNT,
    'collectionsUi.features.edit.otherAccount': EDIT_OTHER_ACCOUNT,
    'collectionsUi.features.redact.accountRequired': NEEDS_ACCOUNT,
    'collectionsUi.features.redact.otherAccount': OTHER_ACCOUNT,
    'collectionsUi.features.text.length':
      'Use at most {characters} characters and {bytes} UTF-8 bytes per message.',
    'collectionsUi.features.text.lineLimit':
      'This network has not supplied a usable message line limit.',
    'collectionsUi.features.text.multilineUnsupported':
      'This network does not support multiline messages. Use one line of at most {bytes} UTF-8 bytes.',
    'collectionsUi.features.text.multilineBytes':
      'This network allows at most {bytes} UTF-8 bytes per multiline message.',
    'collectionsUi.features.text.multilineLines':
      'This network allows at most {lines} wire lines per multiline message.',
  }
  return Object.entries(values ?? {}).reduce(
    (message, [name, value]) => message.replaceAll(`{${name}}`, String(value)),
    messages[key] ?? key,
  )
}

export function conversationSendReason(
  conversation: Conversation | undefined,
  network: Network | undefined,
  live = true,
  translate: MessageReasonTranslator = englishReason,
): string | undefined {
  if (!live || network?.status !== 'connected')
    return translate('collectionsUi.features.send.reconnect')
  if (!conversation || (conversation.kind === 'channel' && !conversation.joined))
    return translate('collectionsUi.features.send.join')
  if (conversation.kind === 'channel' && conversation.modes?.includes('m')) {
    const { modes, prefixes } = parsePrefix(network.isupport)
    const voice = modes.indexOf('v')
    const permitted = [...(conversation.memberModes?.you ?? '')].some((prefix) => {
      const rank = prefixes.indexOf(prefix)
      return voice >= 0 && rank >= 0 && rank <= voice
    })
    if (!permitted) return translate('collectionsUi.features.send.moderated')
  }
}

/** One decision shared by toolbars, context menus and mutation guards. */
export function messageActionReason(
  message: Message,
  conversation: Conversation | undefined,
  network: Network | undefined,
  action: MessageAction,
  live = true,
  translate: MessageReasonTranslator = englishReason,
): string | undefined {
  if (!live || network?.status !== 'connected')
    return translate('collectionsUi.features.action.reconnect')
  if (!conversation || (conversation.kind === 'channel' && !conversation.joined))
    return translate('collectionsUi.features.action.join')
  if (action !== 'redact') {
    const reason = conversationSendReason(conversation, network, live, translate)
    if (reason) return reason
  }
  if (
    !message.upstreamId ||
    message.upstreamId.startsWith('_') ||
    message.delivery ||
    message.deleted ||
    !['message', 'action', 'notice'].includes(message.kind)
  )
    return translate('collectionsUi.features.action.invalidId')
  if (action === 'redact' && message.senderId !== 'you')
    return translate('collectionsUi.features.redact.ownOnly')
  if (action === 'edit') {
    if (message.senderId !== 'you') return translate('collectionsUi.features.edit.ownOnly')
    if (message.kind === 'notice') return translate('collectionsUi.features.edit.notice')
    if (message.gif || message.attachment || message.attachments?.length)
      return translate('collectionsUi.features.edit.media')
  }
  if (!network.features[action]) {
    if (action === 'redact') return translate('collectionsUi.features.redact.unsupported')
    if (action === 'edit') return translate('collectionsUi.features.edit.unsupported')
    return translate(
      action === 'replies'
        ? 'collectionsUi.features.replies.unavailable'
        : 'collectionsUi.features.reactions.unavailable',
    )
  }
  if (action === 'redact') return redactionAccountReason(message, conversation, network, translate)
  if (action === 'edit') return editAccountReason(message, network, translate)
}

/** Stable reason IDs for menus; uses the same policy path as the displayed explanation. */
export function messageActionReasonKey(
  message: Message,
  conversation: Conversation | undefined,
  network: Network | undefined,
  action: MessageAction,
  live = true,
): string | undefined {
  return messageActionReason(message, conversation, network, action, live, (key) => key)
}

export const EDIT_UNSUPPORTED =
  'This network does not support editing messages (it needs message-tags, echo-message and account-tag)'
export const EDIT_NEEDS_ACCOUNT = 'Editing needs a logged-in account on this network'
export const EDIT_OTHER_ACCOUNT =
  'This message wasn’t sent from the account you’re logged into, so other clients would not accept the edit'

/**
 * Other IRCord clients accept an edit only from the account the original was sent from, so
 * unlike deletion there is no operator exception: editing someone else's words is not offered.
 */
function editAccountReason(
  message: Message,
  network: Network,
  translate: MessageReasonTranslator,
): string | undefined {
  const account = network.account?.toLowerCase()
  if (!account) return translate('collectionsUi.features.edit.accountRequired')
  if (message.account?.toLowerCase() !== account)
    return translate('collectionsUi.features.edit.otherAccount')
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
  translate: MessageReasonTranslator,
): string | undefined {
  if (channelRights('you', conversation, network).op) return undefined
  const account = network.account?.toLowerCase()
  if (!account) return translate('collectionsUi.features.redact.accountRequired')
  if (message.account?.toLowerCase() !== account)
    return translate('collectionsUi.features.redact.otherAccount')
}

/** Mirrors the effective byte/line bounds published by Go, after formatting. */
export function messageTextReason(
  text: string,
  kind: 'message' | 'action',
  conversation: Conversation,
  network: Network,
  translate: MessageReasonTranslator = englishReason,
): string | undefined {
  const wire = kind === 'action' ? `\x01ACTION ${text}\x01` : text
  const bytes = new TextEncoder().encode(wire).length
  if ([...text].length > 4000 || new TextEncoder().encode(text).length > 4096)
    return translate('collectionsUi.features.text.length', { characters: 4000, bytes: 4096 })
  const target =
    conversation.kind === 'channel'
      ? channelTarget(conversation)
      : (conversation.target ?? conversation.name)
  const budget =
    network.features.messageLineBytes - new TextEncoder().encode(`PRIVMSG ${target} :`).length
  if (budget < 4) return translate('collectionsUi.features.text.lineLimit')
  if (bytes <= budget && !wire.includes('\n')) return
  const limits = network.features.multiline
  if (!limits)
    return translate('collectionsUi.features.text.multilineUnsupported', { bytes: budget })
  if (bytes > limits.maxBytes)
    return translate('collectionsUi.features.text.multilineBytes', { bytes: limits.maxBytes })
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
    return translate('collectionsUi.features.text.multilineLines', { lines: limits.maxLines })
}
