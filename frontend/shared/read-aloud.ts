// Read-aloud (#201): which incoming messages are spoken, and what is said. The rule is the
// client's own highlight rule (an @mention of your nick or a highlight word), within the notification rules:
// never your own messages, events, ignored senders, muted conversations or quiet hours.
import type { Conversation, Message } from './types/chat'
import { isHighlight, notificationFor, type NotificationSettings } from './notifications'
import { stripFormatting } from './utils/irc'
import type { MentionRecipient } from './group-mentions'

/** Longest text spoken for one message, in characters (requests allow up to 1000). */
export const maxSpokenLength = 300
/** Messages waiting to be read; older ones are dropped when a burst exceeds it. */
export const maxReadAloudQueue = 5

export function shouldReadAloud(
  settings: NotificationSettings,
  c: Pick<Conversation, 'id' | 'kind'> & Partial<Pick<Conversation, 'channelSettings'>>,
  message: Pick<Message, 'text' | 'kind' | 'senderId'>,
  ownNick: string | MentionRecipient | undefined,
  senderNick: string | undefined,
  now = new Date(),
  sender?: { username?: string; host?: string },
) {
  if (!message.text.trim()) return false
  if (notificationFor(settings, c, message, ownNick, senderNick, now, sender) !== 'mention')
    return false
  // A direct message alerts as a mention, but is only read when it highlights you too.
  return isHighlight(settings, message.text, ownNick)
}

/** “alice in #dev: hello there”, without IRC formatting, links shortened and length capped. */
export function spokenText(
  input: {
    sender: string | undefined
    channel?: string
    text: string
    gif?: boolean
  },
  translate?: (key: string, values?: Record<string, string>) => string,
) {
  const who = input.sender?.trim() || translate?.('workspaceUi.speech.someone') || 'Someone'
  const where = input.channel ? ` in ${input.channel}` : ''
  const body = input.gif
    ? (translate?.('workspaceUi.speech.sentGif') ?? 'sent a GIF')
    : stripFormatting(input.text)
        .replace(
          /\bhttps?:\/\/([^\s/]+)\S*/gi,
          (_url, host: string) =>
            translate?.('workspaceUi.speech.linkTo', { host }) ?? `a link to ${host}`,
        )
        .replace(/\s+/g, ' ')
        .trim()
  const line = input.channel
    ? (translate?.('workspaceUi.speech.channelLine', {
        sender: who,
        channel: input.channel,
        body,
      }) ?? `${who}${where}: ${body}`)
    : (translate?.('workspaceUi.speech.line', { sender: who, body }) ?? `${who}: ${body}`)
  return line.length > maxSpokenLength ? `${line.slice(0, maxSpokenLength - 1).trimEnd()}…` : line
}

/** A bounded first-in, first-out list: a burst keeps only the newest `limit` entries. */
export function enqueueBounded<T>(queue: T[], item: T, limit = maxReadAloudQueue) {
  queue.push(item)
  while (queue.length > limit) queue.shift()
  return queue
}
