/**
 * Links to one message in a conversation: `/channel/<id>?m=<msgId>` or `/dm/<id>?m=<msgId>`
 * on the app's own origin. Opening one reveals the message in the reader's own history, so a
 * link works for anyone who has that message stored: everyone on the shared Home network, and
 * the link's maker on their own networks.
 */
export type MessageLink = { conversationId: string; messageId: string }

/** The query parameter that names the message on a conversation route. */
export const messageParam = 'm'

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
