import { mediaLinkText } from './link-media'

/**
 * Links to one message in a conversation: `/channel/<id>?m=<msgId>` or `/dm/<id>?m=<msgId>`
 * on the app's own origin. Opening one finds the message in the reader's own history, by
 * its ID or by its shared ID. Links to channel, group and direct messages on the shared Home
 * network open for everyone who has the message; on networks you added, for you only.
 */
export type MessageLink = { conversationId: string; messageId: string }

/** The query parameter that names the message on a conversation route. */
export const messageParam = 'm'

type LinkableMessage = { id: string; upstreamId?: string }

/**
 * The ID every reader on the network stores for a message with an upstream msgid. The
 * sender's own copy keeps the local ID of the send, so links use this ID when there is one.
 */
export function sharedMessageId(networkId: string, message: LinkableMessage) {
  return message.upstreamId && !message.upstreamId.startsWith('_')
    ? `${networkId}:${message.upstreamId}`
    : ''
}

/** The ID a link to this message carries. */
export function linkMessageId(networkId: string, message: LinkableMessage) {
  return sharedMessageId(networkId, message) || message.id
}

/** Whether a stored message is the one a link names, by either of its IDs. */
export function isLinkedMessage(networkId: string, message: LinkableMessage, id: string) {
  return message.id === id || sharedMessageId(networkId, message) === id
}

/** `path` is the conversation's route, such as `/channel/<id>`. */
export function messageLinkURL(origin: string, path: string, messageId: string) {
  const url = new URL(path, origin)
  url.searchParams.set(messageParam, messageId)
  return url.href
}

/** The conversation and message a URL points to, when it is a message link on `origin`. */
export function parseMessageLink(raw: string, origin: string): MessageLink | null {
  let url: URL
  try {
    url = new URL(raw, origin)
  } catch {
    return null
  }
  if (url.origin !== new URL(origin).origin) return null
  const match = /^\/(?:channel|dm)\/([^/]+)\/?$/.exec(url.pathname)
  const messageId = url.searchParams.get(messageParam)
  if (!match || !messageId) return null
  try {
    const conversationId = decodeURIComponent(match[1]!)
    return conversationId ? { conversationId, messageId } : null
  } catch {
    return null
  }
}

/**
 * A message link in prose, read as the link previews read it: punctuation and unbalanced
 * closing brackets after the link are not part of it (“(see https://…?m=abc).”).
 */
export function messageLinkInText(text: string, origin: string) {
  // eslint-disable-next-line no-control-regex -- control characters end a URL in IRC text
  const url = /^https?:\/\/[^\s<>"'\x00-\x1f]+/.exec(text)?.[0]
  return url ? parseMessageLink(mediaLinkText(url), origin) : null
}
