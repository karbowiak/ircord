export interface User {
  id: string
  username: string
  avatar: string
  status: 'online' | 'away' | 'offline'
}

export type ChannelMode = 'op' | 'voice' | 'regular'

export interface ChannelMember {
  user: User
  mode: ChannelMode
}

export interface Server {
  id: string
  name: string
  abbreviation: string
  channels: Channel[]
}

export interface Channel {
  id: string
  name: string
  type: 'text'
  serverId: string
  topic?: string
  modes?: string
  unread?: boolean
  members: ChannelMember[]
}

export interface DMChannel {
  id: string
  recipient: User
  unread?: boolean
}

export interface Message {
  id: string
  channelId: string
  author: User
  content: string
  timestamp: string
}
