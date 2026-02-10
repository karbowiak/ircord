import { useMockData } from '~/composables/useMockData'

type RequestedSource = 'mock' | 'irc'

export type IrcRuntimeDefaults = {
  webSocketUrl: string
  serverName: string
  nick: string
  username: string
  realname: string
}

function parseBooleanFlag(value: unknown): boolean {
  if (typeof value === 'boolean') {
    return value
  }

  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    return normalized !== 'false' && normalized !== '0' && normalized !== 'off'
  }

  return true
}

export function useChatDataSource() {
  const runtimeConfig = useRuntimeConfig()
  const mockEnabled = parseBooleanFlag(runtimeConfig.public.useMockData)
  const requestedSource: RequestedSource = mockEnabled ? 'mock' : 'irc'
  const ircDefaults: IrcRuntimeDefaults = {
    webSocketUrl: runtimeConfig.public.ircWebSocketUrl,
    serverName: runtimeConfig.public.ircServerName,
    nick: runtimeConfig.public.ircNick,
    username: runtimeConfig.public.ircUsername,
    realname: runtimeConfig.public.ircRealname,
  }

  if (requestedSource === 'mock') {
    return {
      ...useMockData(),
      source: requestedSource,
      requestedSource,
      ircDefaults,
    }
  }

  return {
    users: [],
    servers: [],
    dms: [],
    messages: [],
    source: requestedSource,
    requestedSource,
    ircDefaults,
  }
}
