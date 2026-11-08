// Group mentions (@here, @channel, @everyone, @owner, @operator/@oper, @voiced) are an IRCord
// convention, not an IRC feature: the text goes out as typed and plain IRC clients see words.
// Each IRCord account decides from its own view of the channel whether a token includes it.
// The backend applies the same rule (backend/internal/notify/groups.go) for push and the inbox.
import type { Conversation, Message, Network } from './types/chat'
import { parsePrefix, prefixRank } from './utils/irc'

export type GroupMention = 'here' | 'channel' | 'everyone' | 'owner' | 'operator' | 'voiced'
export type GroupMentionSetting = 'all' | 'ops' | 'off'
/** Reach the whole channel at once, so the channel's setting decides who may use them. */
export const channelWide: readonly GroupMention[] = ['here', 'channel', 'everyone']

/** Offered in the composer's “@” autocomplete, in this order. */
export const groupMentions: { name: GroupMention; description: string }[] = [
  { name: 'here', description: 'Everyone here who isn’t away' },
  { name: 'channel', description: 'Everyone here who isn’t away' },
  { name: 'everyone', description: 'Everyone in the channel' },
  { name: 'owner', description: 'Channel owners' },
  { name: 'operator', description: 'Operators and above' },
  { name: 'voiced', description: 'Voiced members and above' },
]

/** You as a channel sees you: what group mentions need to decide whether they include you. */
export interface MentionRecipient {
  nick: string | undefined
  /** False in direct messages, where group mentions never apply. */
  channel: boolean
  away: boolean
  /** A message from history: @here, @channel and @everyone reached whoever was there then. */
  offline?: boolean
  /**
   * Whether you hold that status or a higher one, by position in the network's PREFIX. A role
   * mention includes the ranks above it: @voiced reaches everyone who can speak in a moderated
   * channel, @operator also reaches admins and owners.
   */
  owner: boolean
  operator: boolean
  voiced: boolean
  /** The channel's setting for @here, @channel and @everyone; missing means 'ops'. */
  groupMentions?: GroupMentionSetting
  /** Whether the sender holds operator status or higher in the channel. */
  senderOperator?: boolean
}

const token = /@(here|channel|everyone|owner|operator|oper|voiced)/gi
const wordChar = /[\w-]/

/** Group mentions in text, whole words only, “@oper” as “operator”. */
export function groupsIn(text: string): GroupMention[] {
  const found: GroupMention[] = []
  for (const match of text.matchAll(token)) {
    const start = match.index
    const end = start + match[0].length
    if (
      (start > 0 && wordChar.test(text[start - 1]!)) ||
      (end < text.length && wordChar.test(text[end]!))
    )
      continue
    const name = match[1]!.toLowerCase()
    found.push((name === 'oper' ? 'operator' : name) as GroupMention)
  }
  return found
}

/** Whether a whole word like “@here” or “@oper” is a group mention. */
export const isGroupMention = (word: string) => groupsIn(word).length === 1 && /^@\w+$/.test(word)

/**
 * Whether a member holds the channel status `mode` (q, o, v…) or a higher one, by position in
 * the network's PREFIX, like the backend's channelAt. False when the network has no such mode.
 */
export function statusAtLeast(
  memberId: string,
  c: Partial<Pick<Conversation, 'memberModes'>>,
  network: Pick<Network, 'isupport'> | undefined,
  mode: string,
) {
  const index = parsePrefix(network?.isupport).modes.indexOf(mode)
  return index >= 0 && prefixRank(c.memberModes?.[memberId] ?? '', network) <= index
}

/** Whether the channel's setting lets someone use @here, @channel and @everyone. */
export const mayReachChannel = (setting: GroupMentionSetting | undefined, operator: boolean) =>
  setting === 'all' || (setting !== 'off' && operator)

/** Whether a group mention in text includes the recipient. */
export function groupMentioned(text: string, r: MentionRecipient) {
  if (!r.channel) return false
  // Channel-wide mentions follow the channel's setting and reach only who is there; role
  // mentions are open to everyone.
  const wide = !r.offline && mayReachChannel(r.groupMentions, !!r.senderOperator)
  return groupsIn(text).some((group) =>
    group === 'everyone'
      ? wide
      : group === 'here' || group === 'channel'
        ? wide && !r.away
        : r[group],
  )
}

/**
 * You in a conversation, for a message: your nick there, your away state, your channel status,
 * the channel's group-mention setting and whether the sender is an operator. Away is your
 * account-wide status; the backend tracks it per network, so with different away states on two
 * networks its counts can briefly differ from this highlight.
 */
export function recipientIn(
  c: Pick<Conversation, 'kind'> & Partial<Pick<Conversation, 'memberModes' | 'channelSettings'>>,
  network: (Partial<Pick<Network, 'nick'>> & Pick<Network, 'isupport'>) | undefined,
  self: { nick?: string; status?: string } | undefined,
  message?: Pick<Message, 'senderId'> & { historical?: boolean },
): MentionRecipient {
  const at = (id: string, mode: string) => statusAtLeast(id, c, network, mode)
  return {
    nick: network?.nick ?? self?.nick,
    channel: c.kind === 'channel',
    away: self?.status === 'away',
    offline: !!message?.historical,
    owner: at('you', 'q'),
    operator: at('you', 'o'),
    voiced: at('you', 'v'),
    groupMentions: c.channelSettings?.groupMentions,
    senderOperator: !!message && at(message.senderId, 'o'),
  }
}
