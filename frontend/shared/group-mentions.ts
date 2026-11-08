// Group mentions (@here, @channel, @everyone, @owner, @operator/@oper, @voiced) are an IRCord
// convention, not an IRC feature: the text goes out as typed and plain IRC clients see words.
// Each IRCord account decides from its own view of the channel whether a token includes it.
// The backend applies the same rule (backend/internal/notify/groups.go) for push and the inbox.
import type { Conversation, Network } from './types/chat'
import { parsePrefix } from './utils/irc'

export type GroupMention = 'here' | 'channel' | 'everyone' | 'owner' | 'operator' | 'voiced'
export type ChannelRank = 'owner' | 'admin' | 'operator' | 'halfop' | 'voiced' | ''

/** Highest first. A role mention includes the ranks above it. */
const ranks: ChannelRank[] = ['owner', 'admin', 'operator', 'halfop', 'voiced']
const rankOfMode: Record<string, ChannelRank> = {
  q: 'owner',
  a: 'admin',
  o: 'operator',
  h: 'halfop',
  v: 'voiced',
}

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
  rank: ChannelRank
  /** The channel's setting for @here, @channel and @everyone; missing means 'ops'. */
  groupMentions?: GroupMentionSetting
  /** Whether the sender holds operator status or higher in the channel. */
  senderOperator?: boolean
}
export type GroupMentionSetting = 'all' | 'ops' | 'off'

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

/** Highest named rank among a member's channel prefixes, using the network's PREFIX. */
export function rankFor(
  prefixes: string,
  isupport: Record<string, string> | undefined,
): ChannelRank {
  const { modes, prefixes: symbols } = parsePrefix(isupport)
  let best: ChannelRank = ''
  for (const char of prefixes) {
    const index = symbols.indexOf(char)
    const rank = index >= 0 ? (rankOfMode[modes[index]!] ?? '') : ''
    if (rank && (!best || ranks.indexOf(rank) < ranks.indexOf(best))) best = rank
  }
  return best
}

/** Whether a group mention in text includes the recipient. */
export function groupMentioned(text: string, r: MentionRecipient) {
  if (!r.channel) return false
  const atLeast = (wanted: ChannelRank) =>
    !!r.rank && ranks.indexOf(r.rank) <= ranks.indexOf(wanted)
  // Channel-wide mentions follow the channel's setting; role mentions are open to everyone.
  const wide = r.groupMentions === 'all' || (r.groupMentions !== 'off' && !!r.senderOperator)
  return groupsIn(text).some((group) =>
    group === 'everyone'
      ? wide
      : group === 'here' || group === 'channel'
        ? wide && !r.away
        : atLeast(group),
  )
}

/**
 * You in a conversation, for a message from `senderId`: your nick there, your away state, your
 * channel status, the channel's group-mention setting and whether the sender is an operator.
 */
export function recipientIn(
  c: Pick<Conversation, 'kind'> & Partial<Pick<Conversation, 'memberModes' | 'channelSettings'>>,
  network: (Partial<Pick<Network, 'nick'>> & Pick<Network, 'isupport'>) | undefined,
  self: { nick?: string; status?: string } | undefined,
  senderId?: string,
): MentionRecipient {
  const rank = (id: string) => rankFor(c.memberModes?.[id] ?? '', network?.isupport)
  const sender = senderId ? rank(senderId) : ''
  return {
    nick: network?.nick ?? self?.nick,
    channel: c.kind === 'channel',
    away: self?.status === 'away',
    rank: rank('you'),
    groupMentions: c.channelSettings?.groupMentions,
    senderOperator: sender === 'owner' || sender === 'admin' || sender === 'operator',
  }
}
