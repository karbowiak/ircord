export interface IrcServerDirectoryEntry {
  id: string
  name: string
  category: string
  country: string
  users: number
  channels: number
  latencyMs: number
  webSocketUrl: string
  serverName: string
  suggestedChannels: string[]
}

export const ircServerDirectory: IrcServerDirectoryEntry[] = [
  {
    id: 'libera',
    name: 'Libera.Chat',
    category: 'General FOSS',
    country: 'Global',
    users: 53000,
    channels: 29000,
    latencyMs: 34,
    webSocketUrl: 'wss://irc.libera.chat/websocket',
    serverName: 'irc.libera.chat',
    suggestedChannels: ['#libera', '#help'],
  },
  {
    id: 'oftc',
    name: 'OFTC',
    category: 'Open Source',
    country: 'Global',
    users: 13000,
    channels: 7000,
    latencyMs: 41,
    webSocketUrl: 'wss://irc.oftc.net:6697',
    serverName: 'irc.oftc.net',
    suggestedChannels: ['#oftc'],
  },
  {
    id: 'snoonet',
    name: 'Snoonet',
    category: 'Community',
    country: 'US',
    users: 12000,
    channels: 5500,
    latencyMs: 97,
    webSocketUrl: 'wss://irc.snoonet.org',
    serverName: 'irc.snoonet.org',
    suggestedChannels: ['#snoonet'],
  },
  {
    id: 'rizon',
    name: 'Rizon',
    category: 'Anime/Gaming',
    country: 'Global',
    users: 16000,
    channels: 9000,
    latencyMs: 88,
    webSocketUrl: 'wss://irc.rizon.net',
    serverName: 'irc.rizon.net',
    suggestedChannels: ['#rizon'],
  },
  {
    id: 'quakenet',
    name: 'QuakeNet',
    category: 'Gaming',
    country: 'EU',
    users: 20000,
    channels: 10000,
    latencyMs: 54,
    webSocketUrl: 'wss://ws.quakenet.org',
    serverName: 'irc.quakenet.org',
    suggestedChannels: ['#quakenet'],
  },
  {
    id: 'ircnet',
    name: 'IRCnet',
    category: 'Classic IRC',
    country: 'EU',
    users: 7000,
    channels: 4500,
    latencyMs: 62,
    webSocketUrl: 'wss://open.ircnet.net',
    serverName: 'open.ircnet.net',
    suggestedChannels: ['#ircnet'],
  },
]
