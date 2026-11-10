import type { components } from '../generated/api'
import type { Body } from '../api/client'
type Schema = components['schemas']
export type Member = Schema['Member'] & { location?: string }

export type NetworkAuth = Schema['NetworkAuth']

export type Network = Schema['Network']

/** What the client sends to add or edit a network. Passwords are write-only. */
export type NetworkInput = Omit<Schema['NetworkConfig'], 'auth' | 'autoJoin' | 'perform'> & { auth: NonNullable<Schema['NetworkConfig']['auth']>; autoJoin: string[]; perform: string[] }

export type Conversation = Omit<Schema['Conversation'], 'joined' | 'memberModes' | 'modes'> & Partial<Pick<Schema['Conversation'], 'joined' | 'memberModes' | 'modes'>>

/** Channel-wide settings shared by every member of a channel; operators change them. */
export type ChannelSettings = Schema['ChannelSettings']
export type ChannelSettingsChange = Body<'/api/v1/conversations/{id}/channel-settings', 'put'>

/** An entry in a channel's ban (+b), exception (+e), invite-exception (+I) or quiet (+q) list. */
export type ChannelListEntry = Schema['ChannelListEntry']

export type Reaction = Schema['Reaction']
export type Reply = Schema['Reply']
export type Attachment = Schema['Attachment']
export type MembershipEvent = Schema['MembershipEvent']
export type Message = Schema['Message'] & { delivery?: 'pending' | 'failed'; liveSeq?: number }

export type ProfileUpdate = Schema['ProfileUpdate']
export type Capabilities = Partial<Schema['Capabilities']>
export type Account = Schema['Account']
export type Upload = Schema['Upload']
export type ServerLine = Schema['ServerLine']
export type ServerLogPage = Schema['ServerLogPage']
export type ConnectionInfo = Schema['ConnectionInfo']
export type ChannelListing = Schema['ChannelListing']
export type Bootstrap = Omit<Schema['Snapshot'], 'capabilities' | 'conversations' | 'members' | 'user'> & { capabilities: Capabilities; conversations: Conversation[]; members: Member[]; user: Member }
/**
 * A conversation's cached timeline. `detached` marks a window opened around an older message
 * (the server said newer messages exist): live messages are not appended until it returns to
 * the present.
 */
export type MessagePage = Omit<Schema['MessagePage'], 'items'> & {
  items: Message[]
  detached?: boolean
  partial?: boolean
  offline?: boolean
  /** Latest skipped live event; an older HTTP read cut cannot prove this window is current. */
  newerLiveSeq?: number
}
export type SearchResult = Schema['SearchResult']
/** One of your private bookmarks, resolved against your current history. */
export type SavedItem = Omit<Schema['SavedItem'], 'message' | 'conversation'> & { message?: Message; conversation?: Conversation }
export type SavedPage = Omit<Schema['SavedPage'], 'items'> & { items: SavedItem[] }
export type SavedRef = Schema['SavedMessage']
export type LinkCard = Schema['LinkCard']
export type LinkSource = Schema['LinkSource']
export type LinkPage = Omit<Schema['LinkPage'], '$schema'>
export type LinkKind = LinkCard['kind']
/** One item of your private attention inbox, resolved against your current history. */
export type InboxItem = Schema['InboxItem']
export type InboxPage = Schema['InboxPage']
export type InboxReason = InboxItem['reasons'][number]
/** The inbox's focused views; dismissed lists what you dismissed, to restore it. */
export type InboxFilter = 'all' | 'unread' | 'mentions' | 'replies' | 'direct' | 'dismissed'

export type TypingState = 'active' | 'paused' | 'done'

export type ClientEvent = Schema['ClientEvent']
  | ({ v: 1; type: 'message.send'; conversationId: string } & Body<'/api/v1/conversations/{id}/messages', 'post'>)
  | ({ v: 1; type: 'raw.send'; networkId: string } & Body<'/api/v1/networks/{id}/commands', 'post'>)

export type ServerEvent = Schema['ServerEvent']
export type DurableEvent = Schema['Event']
