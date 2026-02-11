import { buildCommand, buildTagString, parseIrcMessage, parseNickname } from './protocol'
import type { IrcTransport } from './transport'

export type IrcAdapterOptions = {
  nick: string
  username: string
  realname: string
  password?: string
  transport: IrcTransport
  serverName: string
}

export type IrcIncomingMessageEvent = {
  channelId: string
  author: string
  content: string
  msgid: string | null
  replyToMsgid: string | null
  reaction: string | null
  tags: Record<string, string>
  timestamp: string
}

export type NamesEntry = {
  nick: string
  mode: 'owner' | 'admin' | 'op' | 'halfop' | 'voice' | 'regular'
}

export type HistoryMessage = {
  author: string
  content: string
  msgid: string | null
  replyToMsgid: string | null
  reaction: string | null
  tags: Record<string, string>
  timestamp: string
  channelId: string
}

export type ChannelBanEntry = {
  mask: string
  setBy: string
  setAt: string | null
}

export type WhoisInfo = {
  nick: string
  username: string | null
  hostname: string | null
  realname: string | null
  server: string | null
  serverInfo: string | null
  channels: string[]
  account: string | null
  isOperator: boolean
  idleSeconds: number | null
  signonTime: string | null
  secure: boolean
}

export type RawLogEntry = {
  direction: 'in' | 'out'
  line: string
  time: string
}

type Waiter = {
  matcher: (line: string) => boolean
  resolve: (line: string) => void
  reject: (error: Error) => void
  timeoutId: ReturnType<typeof setTimeout>
}

const MAX_RAW_LOG = 200

const REQUESTED_CAPABILITIES = [
  'message-tags',
  'echo-message',
  'draft/message-redaction',
  'server-time',
  'batch',
  'draft/chathistory',
]

const PREFIX_MODE_MAP: Record<string, NamesEntry['mode']> = {
  '~': 'owner',
  '&': 'admin',
  '@': 'op',
  '%': 'halfop',
  '+': 'voice',
}

export class IrcAdapter {
  private capabilities = new Set<string>()
  private waiters: Waiter[] = []
  private connected = false
  private _nick: string
  private closed = false
  private sawRegistrationSignal = false
  private messageHandlers = new Set<(event: IrcIncomingMessageEvent) => void>()
  private rawLineHandlers = new Set<(entry: RawLogEntry) => void>()

  private _rawLog: RawLogEntry[] = []
  private _latencyMs: number | null = null
  private _userModes = ''
  private _connectedAt: string | null = null
  private _pendingPing: { token: string; sentAt: number } | null = null
  private _latencyInterval: ReturnType<typeof setInterval> | null = null

  // NAMES collection state
  private _namesCollectors = new Map<string, NamesEntry[]>()
  private _joinedChannelNames = new Map<string, NamesEntry[]>()
  private _joinedChannels = new Set<string>()
  private _pendingExplicitJoins = new Set<string>()  // channels being explicitly joined (prevents 366 handler from finalizing)

  // BATCH state for CHATHISTORY
  private _activeBatches = new Map<string, { channel: string; messages: HistoryMessage[] }>()

  constructor(private readonly options: IrcAdapterOptions) {
    this._nick = options.nick
    options.transport.onLine(line => {
      this.logRaw('in', line)
      this.handleLine(line)
    })
    options.transport.onClose(() => {
      this.connected = false
      this.closed = true
      this.stopLatencyPolling()
    })
    options.transport.onError((error) => {
      for (const waiter of this.waiters.splice(0)) {
        clearTimeout(waiter.timeoutId)
        waiter.reject(error)
      }
    })
  }

  // --- Public getters ---

  get currentNick(): string {
    return this._nick
  }

  get userModes(): string {
    return this._userModes
  }

  get latencyMs(): number | null {
    return this._latencyMs
  }

  get rawLog(): ReadonlyArray<RawLogEntry> {
    return this._rawLog
  }

  get connectedAt(): string | null {
    return this._connectedAt
  }

  get negotiatedCapabilities(): string[] {
    return Array.from(this.capabilities)
  }

  // --- Connection lifecycle ---

  async connect(): Promise<void> {
    if (this.connected) return

    await this.options.transport.connect()
    const registrationWaiter = this.waitFor(line => isRegistrationSignal(parseIrcMessage(line), line), 15000)

    const capLsWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      return msg.command === 'CAP' && (msg.params[1] || '').toUpperCase() === 'LS'
    }, 3000)

    this.send('CAP LS 302')
    if (this.options.password) {
      this.send(buildCommand('PASS', [], this.options.password))
    }
    this.send(buildCommand('NICK', [this._nick]))
    this.send(buildCommand('USER', [this.options.username, '0', '*'], this.options.realname))

    let sawCapLs = false
    try {
      await capLsWaiter
      sawCapLs = true
    } catch {
      sawCapLs = false
    }

    if (sawCapLs) {
      await this.requestCapabilities(REQUESTED_CAPABILITIES)
    }

    this.send('CAP END')
    try {
      await registrationWaiter
    } catch (error) {
      if (!this.sawRegistrationSignal) {
        throw error
      }
    }
    this.connected = true
    this.closed = false
    this._connectedAt = new Date().toISOString()

    this.startLatencyPolling()
  }

  disconnect(): void {
    if (this.closed) return
    this.closed = true
    this.stopLatencyPolling()
    this.options.transport.close()
  }

  hasCapability(name: string): boolean {
    return this.capabilities.has(name)
  }

  // --- Event subscriptions ---

  onMessage(handler: (event: IrcIncomingMessageEvent) => void): () => void {
    this.messageHandlers.add(handler)
    return () => this.messageHandlers.delete(handler)
  }

  onRawLine(handler: (entry: RawLogEntry) => void): () => void {
    this.rawLineHandlers.add(handler)
    return () => this.rawLineHandlers.delete(handler)
  }

  // --- Channel operations ---

  async joinChannel(channel: string): Promise<void> {
    const normalized = normalizeChannel(channel)

    // If already joined (e.g. server auto-join via draft/persistence),
    // NAMES were already collected by handleLine — skip the explicit JOIN.
    if (this._joinedChannels.has(normalized)) {
      return
    }

    // Mark as explicit join so handleLine's 366 handler doesn't steal our collector
    this._pendingExplicitJoins.add(normalized)

    // Set up NAMES collector BEFORE sending JOIN so we capture auto-NAMES
    this._namesCollectors.set(normalized, [])

    const joinWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      const joined = msg.trailing || msg.params[0] || ''
      return msg.command === 'JOIN' && parseNickname(msg.prefix) === this._nick && joined === normalized
    }, 5000)

    const namesEndWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      return msg.command === '366' && (msg.params[1] || '') === normalized
    }, 10000)

    this.send(buildCommand('JOIN', [normalized]))
    await joinWaiter

    // Wait for auto-NAMES to complete (server always sends 353/366 after JOIN)
    try { await namesEndWaiter } catch {}

    this._pendingExplicitJoins.delete(normalized)
    this._joinedChannels.add(normalized)

    // Store auto-NAMES for getJoinNames()
    const autoNames = this._namesCollectors.get(normalized) || []
    this._namesCollectors.delete(normalized)
    this._joinedChannelNames.set(normalized, autoNames)
  }

  async createChannel(channel: string): Promise<void> {
    await this.joinChannel(channel)
  }

  /** Return NAMES entries collected during joinChannel (one-time read). */
  getJoinNames(channel: string): NamesEntry[] {
    const normalized = normalizeChannel(channel)
    const entries = this._joinedChannelNames.get(normalized) || []
    this._joinedChannelNames.delete(normalized)
    return entries
  }

  // --- NAMES ---

  async requestNames(channel: string): Promise<NamesEntry[]> {
    const normalized = normalizeChannel(channel)
    this._pendingExplicitJoins.add(normalized)  // prevent handleLine 366 from stealing
    this._namesCollectors.set(normalized, [])

    const endWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      return msg.command === '366' && (msg.params[1] || '') === normalized
    }, 10000)

    this.send(buildCommand('NAMES', [normalized]))
    await endWaiter

    this._pendingExplicitJoins.delete(normalized)
    const entries = this._namesCollectors.get(normalized) || []
    this._namesCollectors.delete(normalized)
    return entries
  }

  // --- CHATHISTORY ---

  async requestHistory(channel: string, limit = 100, since?: { msgid?: string; timestamp?: string }): Promise<HistoryMessage[]> {
    if (!this.capabilities.has('draft/chathistory') || !this.capabilities.has('batch')) {
      return []
    }

    const normalized = normalizeChannel(channel)

    // BATCH +ref is handled synchronously in handleLine — it sets up _activeBatches.
    // We just need to wait for BATCH -ref (end of batch) for our channel.
    const endWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      if (msg.command !== 'BATCH') return false
      const ref = msg.params[0] || ''
      if (!ref.startsWith('-')) return false
      const batchRef = ref.slice(1)
      const batch = this._activeBatches.get(batchRef)
      return batch !== undefined && batch.channel === normalized
    }, 15000)

    // Build the cursor: msgid reference, timestamp reference, or wildcard
    let cursor = '*'
    if (since?.msgid) {
      cursor = `msgid=${since.msgid}`
    } else if (since?.timestamp) {
      cursor = `timestamp=${since.timestamp}`
    }

    this.send(buildCommand('CHATHISTORY', ['LATEST', normalized, cursor, String(limit)]))

    try {
      const endLine = await endWaiter
      const endMsg = parseIrcMessage(endLine)
      const ref = (endMsg.params[0] || '').slice(1)
      const batch = this._activeBatches.get(ref)
      this._activeBatches.delete(ref)
      return batch?.messages || []
    } catch {
      return []
    }
  }

  // --- Messaging ---

  async sendMessage(channel: string, content: string): Promise<string | null> {
    return this.sendMessageWithTags(channel, content, {})
  }

  async sendMessageWithTags(
    channel: string,
    content: string,
    tags: Record<string, string>,
  ): Promise<string | null> {
    const normalized = normalizeChannel(channel)
    const text = content.trim()
    if (!text) return null

    const echoWaiter = this.waitForMessage(normalized, (event) => event.author === this._nick && event.content === text)
    const tagPrefix = buildTagString(tags)
    this.send(`${tagPrefix}${buildCommand('PRIVMSG', [normalized], text)}`)
    const echoed = await echoWaiter
    return echoed.msgid
  }

  async replyToMessage(channel: string, parentMsgid: string, content: string): Promise<string | null> {
    const normalized = normalizeChannel(channel)
    const text = content.trim()
    if (!text) return null

    const echoWaiter = this.waitForMessage(
      normalized,
      (event) => event.author === this._nick && event.content === text && event.replyToMsgid === parentMsgid,
    )
    const tagPrefix = buildTagString({ '+draft/reply': parentMsgid, '+reply': parentMsgid })
    this.send(`${tagPrefix}${buildCommand('PRIVMSG', [normalized], text)}`)
    const echoed = await echoWaiter
    return echoed.msgid
  }

  async deleteMessage(channel: string, msgid: string, reason = 'deleted'): Promise<boolean> {
    if (!this.capabilities.has('draft/message-redaction')) {
      return false
    }

    const normalized = normalizeChannel(channel)
    const redactWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      if (msg.command !== 'REDACT') return false
      return (msg.params[0] || '') === normalized && (msg.params[1] || '') === msgid
    }, 5000)

    this.send(buildCommand('REDACT', [normalized, msgid], reason))

    try {
      await redactWaiter
      return true
    } catch {
      return false
    }
  }

  async editMessage(channel: string, msgid: string, content: string): Promise<string | null> {
    const deleted = await this.deleteMessage(channel, msgid, 'edited')
    if (!deleted) {
      return this.sendMessage(channel, content)
    }
    return this.sendMessage(channel, content)
  }

  async setTopic(channel: string, topic: string): Promise<boolean> {
    const normalized = normalizeChannel(channel)
    const nextTopic = topic.trim()

    const topicWaiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === 'TOPIC') {
        return (msg.params[0] || '') === normalized && parseNickname(msg.prefix) === this._nick
      }

      if (msg.command === '482' || msg.command === '442' || msg.command === '403') {
        return (msg.params[1] || '') === normalized
      }

      return false
    }, 5000)

    this.send(buildCommand('TOPIC', [normalized], nextTopic))

    try {
      const line = await topicWaiter
      const msg = parseIrcMessage(line)
      return msg.command === 'TOPIC'
    } catch {
      return false
    }
  }

  async requestChannelTopic(channel: string): Promise<string> {
    const normalized = normalizeChannel(channel)
    const waiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === '332' || msg.command === '331') {
        return (msg.params[1] || '') === normalized
      }
      return false
    }, 5000)

    this.send(buildCommand('TOPIC', [normalized]))

    try {
      const line = await waiter
      const msg = parseIrcMessage(line)
      if (msg.command === '332') {
        return msg.trailing || msg.params.slice(2).join(' ').trim()
      }
      return ''
    } catch {
      return ''
    }
  }

  async requestChannelModes(channel: string): Promise<string> {
    const normalized = normalizeChannel(channel)
    const waiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === '324') {
        return (msg.params[1] || '') === normalized
      }
      if (msg.command === '403' || msg.command === '442') {
        return (msg.params[1] || '') === normalized
      }
      return false
    }, 5000)

    this.send(buildCommand('MODE', [normalized]))

    try {
      const line = await waiter
      const msg = parseIrcMessage(line)
      if (msg.command !== '324') return ''
      return msg.params[2] || ''
    } catch {
      return ''
    }
  }

  async setChannelModes(channel: string, modeSpec: string): Promise<boolean> {
    const normalized = normalizeChannel(channel)
    const trimmed = modeSpec.trim()
    if (!trimmed) return false

    const modeParts = trimmed.split(/\s+/).filter(Boolean)
    if (!modeParts.length) return false

    const waiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === 'MODE') {
        return (msg.params[0] || '') === normalized
      }

      if (msg.command === '482' || msg.command === '442' || msg.command === '403' || msg.command === '472') {
        return (msg.params[1] || '') === normalized
      }

      return false
    }, 5000)

    this.send(buildCommand('MODE', [normalized, ...modeParts]))

    try {
      const line = await waiter
      const msg = parseIrcMessage(line)
      return msg.command === 'MODE'
    } catch {
      return false
    }
  }

  async renameChannel(oldChannel: string, newChannel: string): Promise<boolean> {
    const oldName = normalizeChannel(oldChannel)
    const newName = normalizeChannel(newChannel)
    if (oldName === newName) return true

    const waiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === 'RENAME') {
        return (msg.params[0] || '') === oldName && (msg.params[1] || '') === newName
      }

      if (msg.command === '482' || msg.command === '442' || msg.command === '403' || msg.command === '696') {
        return (msg.params[1] || '') === oldName || (msg.params[1] || '') === newName
      }

      return false
    }, 6000)

    this.send(buildCommand('RENAME', [oldName, newName]))

    try {
      const line = await waiter
      const msg = parseIrcMessage(line)
      return msg.command === 'RENAME'
    } catch {
      return false
    }
  }

  async requestBanList(channel: string): Promise<ChannelBanEntry[]> {
    const normalized = normalizeChannel(channel)
    const entries: ChannelBanEntry[] = []

    const doneWaiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === '367' && (msg.params[1] || '') === normalized) {
        const mask = msg.params[2] || ''
        const setBy = msg.params[3] || ''
        const setAtRaw = msg.params[4] || ''
        entries.push({
          mask,
          setBy,
          setAt: toIsoOrNull(setAtRaw),
        })
      }

      if (msg.command === '368') {
        return (msg.params[1] || '') === normalized
      }

      if (msg.command === '482' || msg.command === '442' || msg.command === '403') {
        return (msg.params[1] || '') === normalized
      }

      return false
    }, 7000)

    this.send(buildCommand('MODE', [normalized, '+b']))

    try {
      await doneWaiter
      return entries
    } catch {
      return []
    }
  }

  async addReaction(channel: string, msgid: string, emoji: string): Promise<boolean> {
    const normalized = normalizeChannel(channel)
    const reactionWaiter = this.waitForMessage(
      normalized,
      event => event.author === this._nick && event.reaction === emoji && event.replyToMsgid === msgid,
    )
    const tagPrefix = buildTagString({
      '+draft/reply': msgid,
      '+reply': msgid,
      '+draft/react': emoji,
      '+react': emoji,
    })
    this.send(`${tagPrefix}${buildCommand('TAGMSG', [normalized])}`)
    try {
      await reactionWaiter
      return true
    } catch {
      return false
    }
  }

  async kickUser(channel: string, nick: string, reason = 'kicked'): Promise<boolean> {
    const normalized = normalizeChannel(channel)
    const kickWaiter = this.waitFor(line => {
      const msg = parseIrcMessage(line)
      return msg.command === 'KICK' && (msg.params[0] || '') === normalized && (msg.params[1] || '') === nick
    }, 5000)

    this.send(buildCommand('KICK', [normalized, nick], reason))
    try {
      await kickWaiter
      return true
    } catch {
      return false
    }
  }

  async banUser(channel: string, nickOrMask: string): Promise<boolean> {
    const normalized = normalizeChannel(channel)
    const trimmed = nickOrMask.trim()
    if (!trimmed) return false

    const mask = trimmed.includes('!') || trimmed.includes('@') ? trimmed : `${trimmed}!*@*`
    const modeWaiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === 'MODE') {
        return (msg.params[0] || '') === normalized
          && (msg.params[1] || '') === '+b'
          && (msg.params[2] || '') === mask
      }

      if (msg.command === '482' || msg.command === '442' || msg.command === '403') {
        return (msg.params[1] || '') === normalized
      }

      return false
    }, 5000)

    this.send(buildCommand('MODE', [normalized, '+b', mask]))
    try {
      const line = await modeWaiter
      const msg = parseIrcMessage(line)
      return msg.command === 'MODE'
    } catch {
      return false
    }
  }

  async setBanMask(channel: string, mask: string): Promise<boolean> {
    const normalized = normalizeChannel(channel)
    const banMask = mask.trim()
    if (!banMask) return false

    const modeWaiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command === 'MODE') {
        return (msg.params[0] || '') === normalized
          && (msg.params[1] || '') === '+b'
          && (msg.params[2] || '') === banMask
      }

      if (msg.command === '482' || msg.command === '442' || msg.command === '403') {
        return (msg.params[1] || '') === normalized
      }

      return false
    }, 5000)

    this.send(buildCommand('MODE', [normalized, '+b', banMask]))
    try {
      const line = await modeWaiter
      const msg = parseIrcMessage(line)
      return msg.command === 'MODE'
    } catch {
      return false
    }
  }

  async slapWithTrout(channel: string, nick: string): Promise<string | null> {
    const text = `\u0001ACTION slaps ${nick} around a bit with a large trout\u0001`
    return this.sendMessage(channel, text)
  }

  sendServiceCommand(serviceNick: string, command: string): void {
    const target = serviceNick.trim()
    const payload = command.trim()
    if (!target || !payload) return
    this.send(buildCommand('PRIVMSG', [target], payload))
  }

  async waitForServiceNotice(
    serviceNick: string,
    successPattern: RegExp,
    timeout = 6000,
    failurePattern?: RegExp,
  ): Promise<boolean> {
    const target = serviceNick.trim().toLowerCase()
    const waiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      if (msg.command !== 'NOTICE') return false
      if ((msg.params[0] || '').toLowerCase() !== this._nick.toLowerCase()) return false
      if (parseNickname(msg.prefix).toLowerCase() !== target) return false

      const text = msg.trailing || msg.params.slice(1).join(' ')
      if (failurePattern?.test(text)) return true
      return successPattern.test(text)
    }, timeout)

    try {
      const line = await waiter
      const msg = parseIrcMessage(line)
      const text = msg.trailing || msg.params.slice(1).join(' ')
      if (failurePattern?.test(text)) return false
      return successPattern.test(text)
    } catch {
      return false
    }
  }

  async requestWhois(nick: string): Promise<WhoisInfo | null> {
    const targetNick = nick.trim()
    if (!targetNick) return null

    const result: WhoisInfo = {
      nick: targetNick,
      username: null,
      hostname: null,
      realname: null,
      server: null,
      serverInfo: null,
      channels: [],
      account: null,
      isOperator: false,
      idleSeconds: null,
      signonTime: null,
      secure: false,
    }

    const doneWaiter = this.waitFor((line) => {
      const msg = parseIrcMessage(line)
      const code = msg.command

      if (code === '401') {
        return (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()
      }

      if (code === '311' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        result.nick = msg.params[1] || result.nick
        result.username = msg.params[2] || null
        result.hostname = msg.params[3] || null
        result.realname = msg.trailing || null
      }

      if (code === '312' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        result.server = msg.params[2] || null
        result.serverInfo = msg.trailing || null
      }

      if (code === '313' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        result.isOperator = true
      }

      if (code === '317' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        const idle = Number(msg.params[2] || '')
        const signon = Number(msg.params[3] || '')
        if (Number.isFinite(idle)) result.idleSeconds = idle
        if (Number.isFinite(signon) && signon > 0) {
          result.signonTime = new Date(signon * 1000).toISOString()
        }
      }

      if (code === '319' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        result.channels = (msg.trailing || '')
          .split(/\s+/)
          .map(entry => entry.trim())
          .filter(Boolean)
      }

      if (code === '330' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        result.account = msg.params[2] || null
      }

      if (code === '671' && (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()) {
        result.secure = true
      }

      if (code === '318') {
        return (msg.params[1] || '').toLowerCase() === targetNick.toLowerCase()
      }

      return false
    }, 7000)

    this.send(buildCommand('WHOIS', [targetNick]))

    try {
      const line = await doneWaiter
      const msg = parseIrcMessage(line)
      if (msg.command === '401') return null
      return result
    } catch {
      return null
    }
  }

  // --- Latency ---

  measureLatency(): void {
    if (!this.connected || this.closed) return
    const token = `ircord-${Date.now()}`
    this._pendingPing = { token, sentAt: Date.now() }
    this.send(buildCommand('PING', [], token))
  }

  // --- Private: send + log ---

  private send(line: string): void {
    this.logRaw('out', line)
    this.options.transport.send(line)
  }

  private logRaw(direction: 'in' | 'out', line: string): void {
    const entry: RawLogEntry = {
      direction,
      line,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    }
    this._rawLog.push(entry)
    if (this._rawLog.length > MAX_RAW_LOG) {
      this._rawLog.splice(0, this._rawLog.length - MAX_RAW_LOG)
    }
    for (const handler of this.rawLineHandlers) {
      handler(entry)
    }
  }

  // --- Private: latency polling ---

  private startLatencyPolling(): void {
    this.stopLatencyPolling()
    this.measureLatency()
    this._latencyInterval = setInterval(() => this.measureLatency(), 30000)
  }

  private stopLatencyPolling(): void {
    if (this._latencyInterval) {
      clearInterval(this._latencyInterval)
      this._latencyInterval = null
    }
  }

  // --- Private: capability negotiation ---

  private async requestCapabilities(names: string[]): Promise<void> {
    if (names.length === 0) return

    for (const name of names) {
      try {
        const capWaiter = this.waitForCapResponse(name)
        this.send(buildCommand('CAP', ['REQ'], name))
        const result = await capWaiter

        if (result.status === 'ACK') {
          this.capabilities.add(name)
        }
      } catch {
      }
    }
  }

  private waitForCapResponse(name: string): Promise<{ status: 'ACK' | 'NAK' }> {
    return new Promise((resolve, reject) => {
      const waiter = this.waitFor(line => {
        const msg = parseIrcMessage(line)
        if (msg.command !== 'CAP') return false

        const status = (msg.params[1] || '').toUpperCase()
        if (status !== 'ACK' && status !== 'NAK') return false

        const capPayload = msg.trailing || msg.params.slice(2).join(' ')
        const caps = capPayload
          .split(' ')
          .filter(Boolean)
          .map(cap => cap.replace(/^[~-]/, ''))
        if (!caps.includes(name)) return false

        resolve({ status: status as 'ACK' | 'NAK' })
        return true
      }, 5000)

      waiter.catch(reject)
    })
  }

  // --- Private: line handling ---

  private handleLine(line: string): void {
    const msg = parseIrcMessage(line)

    if (isRegistrationSignal(msg, line)) {
      this.sawRegistrationSignal = true
    }

    // PING → PONG
    if (msg.command === 'PING') {
      this.send(buildCommand('PONG', [], msg.trailing || msg.params[0] || this.options.serverName))
      return
    }

    // PONG → latency measurement
    if (msg.command === 'PONG' && this._pendingPing) {
      const pongToken = msg.trailing || msg.params[1] || ''
      if (pongToken === this._pendingPing.token) {
        this._latencyMs = Date.now() - this._pendingPing.sentAt
        this._pendingPing = null
      }
    }

    // NICK change
    if (msg.command === 'NICK' && parseNickname(msg.prefix) === this._nick) {
      this._nick = msg.trailing || msg.params[0] || this._nick
    }

    // RPL_UMODEIS (221)
    if (msg.command === '221') {
      this._userModes = msg.params[1] || msg.trailing || ''
    }

    // Track our own JOINs (catches server auto-joins from draft/persistence)
    if (msg.command === 'JOIN' && parseNickname(msg.prefix) === this._nick) {
      const channel = msg.trailing || msg.params[0] || ''
      if (channel && !this._namesCollectors.has(channel)) {
        // Auto-create NAMES collector so the 353/366 that follow are captured
        this._namesCollectors.set(channel, [])
      }
    }

    // Track PART/KICK so we can re-join correctly
    if (msg.command === 'PART' && parseNickname(msg.prefix) === this._nick) {
      const channel = msg.params[0] || ''
      this._joinedChannels.delete(channel)
      this._joinedChannelNames.delete(channel)
    }
    if (msg.command === 'KICK') {
      const kicked = msg.params[1] || ''
      if (kicked === this._nick) {
        const channel = msg.params[0] || ''
        this._joinedChannels.delete(channel)
        this._joinedChannelNames.delete(channel)
      }
    }

    // RPL_NAMREPLY (353) — collect NAMES entries
    if (msg.command === '353') {
      const channel = msg.params[2] || ''
      const collector = this._namesCollectors.get(channel)
      if (collector) {
        const nicks = (msg.trailing || '').split(' ').filter(Boolean)
        for (const raw of nicks) {
          collector.push(parseNamesEntry(raw))
        }
      }
    }

    // RPL_ENDOFNAMES (366) — finalize NAMES collection for auto-joined channels
    // Skip if joinChannel is actively managing this channel (it has its own 366 waiter)
    if (msg.command === '366') {
      const channel = msg.params[1] || ''
      if (!this._pendingExplicitJoins.has(channel)) {
        const collector = this._namesCollectors.get(channel)
        if (collector) {
          this._joinedChannelNames.set(channel, [...collector])
          this._namesCollectors.delete(channel)
          this._joinedChannels.add(channel)
        }
      }
    }

    // BATCH +ref — set up collector synchronously so batched PRIVMSGs are captured
    if (msg.command === 'BATCH') {
      const ref = msg.params[0] || ''
      if (ref.startsWith('+')) {
        const batchRef = ref.slice(1)
        const batchType = msg.params[1] || ''
        if (batchType === 'chathistory') {
          const batchChannel = msg.params[2] || ''
          this._activeBatches.set(batchRef, { channel: batchChannel, messages: [] })
        }
      }
    }

    // Batched PRIVMSG for CHATHISTORY
    if (msg.tags.batch && msg.command === 'PRIVMSG') {
      const batchRef = msg.tags.batch
      const batch = this._activeBatches.get(batchRef)
      if (batch) {
        const author = parseNickname(msg.prefix)

        const serverTime = msg.tags.time || ''
        const timestamp = serverTime
          ? formatServerTime(serverTime)
          : new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

        batch.messages.push({
          channelId: msg.params[0] || '',
          author,
          content: msg.trailing,
          msgid: msg.tags.msgid || null,
          replyToMsgid: msg.tags['+draft/reply'] || msg.tags.reply || null,
          reaction: msg.tags['+draft/react'] || msg.tags.react || null,
          tags: msg.tags,
          timestamp,
        })
        return // don't emit as live message
      }
    }

    // Batched TAGMSG for CHATHISTORY (reactions in history)
    if (msg.tags.batch && msg.command === 'TAGMSG') {
      const batchRef = msg.tags.batch
      const batch = this._activeBatches.get(batchRef)
      if (batch) {
        const serverTime = msg.tags.time || ''
        const timestamp = serverTime
          ? formatServerTime(serverTime)
          : new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

        batch.messages.push({
          channelId: msg.params[0] || '',
          author: parseNickname(msg.prefix),
          content: '',
          msgid: msg.tags.msgid || null,
          replyToMsgid: msg.tags['+draft/reply'] || msg.tags.reply || null,
          reaction: msg.tags['+draft/react'] || msg.tags.react || null,
          tags: msg.tags,
          timestamp,
        })
        return
      }
    }

    // Live PRIVMSG / TAGMSG
    if (msg.command === 'PRIVMSG' || msg.command === 'TAGMSG') {
      const channelId = msg.params[0] || ''
      const serverTime = msg.tags.time || ''
      const timestamp = serverTime
        ? formatServerTime(serverTime)
        : new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

      const event: IrcIncomingMessageEvent = {
        channelId,
        author: parseNickname(msg.prefix),
        content: msg.trailing,
        msgid: msg.tags.msgid || null,
        replyToMsgid: msg.tags['+draft/reply'] || msg.tags.reply || null,
        reaction: msg.tags['+draft/react'] || msg.tags.react || null,
        tags: msg.tags,
        timestamp,
      }
      for (const handler of this.messageHandlers) {
        handler(event)
      }
    }

    for (let i = 0; i < this.waiters.length; i += 1) {
      const waiter = this.waiters[i]
      if (!waiter.matcher(line)) continue

      clearTimeout(waiter.timeoutId)
      this.waiters.splice(i, 1)
      waiter.resolve(line)
      return
    }
  }

  // --- Private: waiters ---

  private waitFor(matcher: (line: string) => boolean, timeoutMs: number): Promise<string> {
    return new Promise((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        const index = this.waiters.findIndex(waiter => waiter.timeoutId === timeoutId)
        if (index >= 0) {
          this.waiters.splice(index, 1)
        }
        reject(new Error(`Timed out waiting for IRC condition after ${timeoutMs}ms`))
      }, timeoutMs)

      this.waiters.push({ matcher, resolve, reject, timeoutId })
    })
  }

  private waitForMessage(
    channel: string,
    matcher: (event: IrcIncomingMessageEvent) => boolean,
    timeoutMs = 5000,
  ): Promise<IrcIncomingMessageEvent> {
    return new Promise((resolve, reject) => {
      const deadline = setTimeout(() => {
        dispose()
        reject(new Error(`Timed out waiting for IRC message in ${channel}`))
      }, timeoutMs)

      const dispose = this.onMessage((event) => {
        if (event.channelId !== channel) return
        if (!matcher(event)) return

        clearTimeout(deadline)
        dispose()
        resolve(event)
      })
    })
  }
}

// --- Helpers ---

function isRegistrationSignal(msg: ReturnType<typeof parseIrcMessage>, rawLine: string): boolean {
  if (msg.command === '001' || msg.command === '376' || msg.command === '422') {
    return true
  }

  if (/\s001\s/.test(rawLine) || /\s376\s/.test(rawLine) || /\s422\s/.test(rawLine)) {
    return true
  }

  if (!/^\d{3}$/.test(msg.command)) {
    return false
  }

  const numeric = Number(msg.command)
  if (Number.isNaN(numeric)) return false

  return numeric >= 1 && numeric < 400
}

function normalizeChannel(channel: string): string {
  const value = channel.trim()
  if (!value) {
    throw new Error('Channel name is required')
  }

  return value.startsWith('#') ? value : `#${value}`
}

function parseNamesEntry(raw: string): NamesEntry {
  const first = raw[0]
  const mode = PREFIX_MODE_MAP[first]
  if (mode) {
    return { nick: raw.slice(1), mode }
  }
  return { nick: raw, mode: 'regular' }
}

function formatServerTime(iso: string): string {
  try {
    const date = new Date(iso)
    if (Number.isNaN(date.getTime())) {
      return iso
    }
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  } catch {
    return iso
  }
}

function toIsoOrNull(value: string): string | null {
  const unix = Number(value)
  if (Number.isFinite(unix) && unix > 0) {
    return new Date(unix * 1000).toISOString()
  }

  const parsed = new Date(value)
  if (!Number.isNaN(parsed.getTime())) {
    return parsed.toISOString()
  }

  return null
}
