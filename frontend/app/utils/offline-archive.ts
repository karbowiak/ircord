import { shallowRef, toRaw } from 'vue'
import type {
  Bootstrap,
  Message,
  MessagePage,
  SearchResult,
  ServerEvent,
} from '../../shared/types/chat'
import type { SearchFilters } from '../../shared/utils/search-filter'
import { searchParams } from '../../shared/utils/search-filter'
import type { OfflineFile, OfflineStatus } from '../../shared/offline'
import { apiClient, errorStatus } from './api'
import { boundaryMessage } from '../../shared/utils/history-cursor'
import { createOfflineFileReads } from './offline-file-reads'

export const offlineStatus = shallowRef<OfflineStatus>({
  phase: 'idle',
  offline: false,
  messages: 0,
  files: 0,
  bytes: 0,
})
export const offlineRevision = shallowRef(0)
/** Changes synchronously when saved bytes from the previous account must be released. */
export const offlineAccountRevision = shallowRef(0)
type ArchiveInfo = {
  meta?: {
    state: Bootstrap
    complete: boolean
    since: string
    updatedAt?: string
    generation: string
  }
  messages: number
  files: number
  bytes: number
}
type FileInfo = { blob?: Blob; refs: { id: string; pinned: boolean }[]; failed?: boolean }
const offlineErrors = {
  offlineUnavailable: 'Offline storage is unavailable',
  offlineRequestFailed: 'Offline request could not be sent',
  offlineAccountChanged: 'Offline account changed',
  offlineStartFailed: 'Offline storage could not start.',
  offlineBrowserUnavailable: 'Offline storage is unavailable in this browser.',
  offlineNeedsSync: 'Offline history needs to synchronize again.',
  offlineMissingState: 'Offline snapshot is missing state',
  offlinePartial: 'Some history is not available offline yet.',
  offlineFileFailed: 'This file could not be saved offline.',
  offlineTransactionFailed: 'Offline storage could not complete the operation.',
  offlineClosed: 'Offline storage is closed.',
  offlineSignedOut: 'The offline account was signed out.',
  offlineSuperseded: 'A newer offline snapshot replaced this one.',
  offlineConversationUnavailable: 'This conversation is not available offline.',
  offlineHistoryExpired: 'Offline history has changed. Try loading it again.',
  offlineMessageUnavailable: 'This message is not available offline.',
  offlineSearchExpired: 'Offline search results have changed. Try searching again.',
  offlineOperationFailed: 'Offline storage could not complete the operation.',
} as const
type OfflineErrorKey = keyof typeof offlineErrors
// The worker RPC boundary is untyped; each caller names the result shape it expects.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type RPC = <T = any>(method: string, ...args: unknown[]) => Promise<T>
export interface LocalSearchPage {
  items: SearchResult[]
  nextCursor: string | null
  offline?: boolean
  since?: string
  updatedAt?: string
}

class OfflineArchive {
  private translate?: (key: string) => string
  private statusErrorKey?: OfflineErrorKey
  private worker?: Worker
  private owner?: string
  private epoch = 0
  private serial = 0
  private pending = new Map<
    number,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    { resolve: (value: any) => void; reject: (error: Error) => void }
  >()
  private controller?: AbortController
  private events: ServerEvent[] = []
  private syncing = false
  private dirty = false
  private flushing = false
  private downloading = false
  private timer?: ReturnType<typeof setTimeout>
  private peerTimer?: ReturnType<typeof setTimeout>
  private refreshTimer?: ReturnType<typeof setInterval>
  private apiBase = '/api/v1'
  private revoked = new Set<string>()
  private channel?: BroadcastChannel
  private online = true
  private fileReads = createOfflineFileReads<FileInfo | undefined>((url) =>
    this.session()<FileInfo | undefined>('file', url),
  )
  private fileSubscribers = new Map<string, Set<() => void>>()

  /** Subscribers exist only while a caller needs this URL; no saved Blob is cached here. */
  subscribeFile(url: string, listener: () => void) {
    let listeners = this.fileSubscribers.get(url)
    if (!listeners) this.fileSubscribers.set(url, (listeners = new Set()))
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
      if (!listeners.size && this.fileSubscribers.get(url) === listeners)
        this.fileSubscribers.delete(url)
    }
  }
  private invalidateFiles(url?: string) {
    this.fileReads.invalidate(url)
    if (url === undefined) offlineRevision.value++
    else for (const listener of [...(this.fileSubscribers.get(url) ?? [])]) listener()
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private rpc<T = any>(method: string, ...args: unknown[]): Promise<T> {
    if (!this.worker) return Promise.reject(new Error(this.message('offlineUnavailable')))
    const id = ++this.serial
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      try {
        this.worker!.postMessage({ id, method, args: args.map((value) => toRaw(value)) })
      } catch (error) {
        this.pending.delete(id)
        reject(error instanceof Error ? error : new Error(this.message('offlineRequestFailed')))
      }
    })
  }
  setTranslator(translate: (key: string) => string) {
    this.translate = translate
    this.refreshError()
  }
  refreshError() {
    if (this.statusErrorKey)
      offlineStatus.value = { ...offlineStatus.value, error: this.message(this.statusErrorKey) }
  }
  private message(key: OfflineErrorKey) {
    return this.translate?.(`workspaceUi.errors.${key}`) ?? offlineErrors[key]
  }
  private patch(values: Partial<OfflineStatus>, errorKey?: OfflineErrorKey) {
    if ('error' in values) this.statusErrorKey = errorKey
    offlineStatus.value = { ...offlineStatus.value, ...values }
  }
  // A task keeps its worker identity across every await. An old download or
  // snapshot must never write into a newly signed-in account's database.
  private session(): RPC {
    const worker = this.worker
    return (method, ...args) =>
      worker && worker === this.worker
        ? this.rpc(method, ...args)
        : Promise.reject(new Error(this.message('offlineAccountChanged')))
  }
  async start(owner: string, apiBase: string) {
    if (this.owner === owner || this.revoked.has(owner)) return
    this.stop()
    this.owner = owner
    this.apiBase = apiBase
    const epoch = this.epoch
    try {
      this.worker = new Worker(new URL('../workers/offline.worker.ts', import.meta.url), {
        type: 'module',
      })
      this.worker.onmessage = ({ data }) => {
        if (epoch !== this.epoch) return
        const pending = this.pending.get(data.id)
        if (!pending) return
        this.pending.delete(data.id)
        if (data.error)
          pending.reject(
            new Error(
              typeof data.errorKey === 'string' && Object.hasOwn(offlineErrors, data.errorKey)
                ? this.message(data.errorKey as OfflineErrorKey)
                : data.error,
            ),
          )
        else pending.resolve(data.result)
      }
      this.worker.onerror = () => {
        if (epoch !== this.epoch) return
        this.stop()
        this.patch(
          { phase: 'unavailable', error: this.message('offlineStartFailed') },
          'offlineStartFailed',
        )
      }
      if ('BroadcastChannel' in window) {
        this.channel = new BroadcastChannel('ircord-offline-accounts')
        this.channel.onmessage = ({ data }) => {
          if (data?.clear === this.owner) {
            this.revoked.add(data.clear)
            this.stop()
          } else if (data?.updated === this.owner) {
            clearTimeout(this.peerTimer)
            const rpc = this.session()
            const epoch = this.epoch
            this.peerTimer = setTimeout(() => {
              void this.refreshInfo(rpc)
                .then(() => {
                  if (epoch === this.epoch) this.invalidateFiles()
                })
                .catch(() => {})
            }, 100)
          }
        }
      }
      const info = await this.rpc<ArchiveInfo>('open', owner)
      if (epoch !== this.epoch) return
      this.updateInfo(info)
      this.invalidateFiles()
      this.refreshTimer = setInterval(() => {
        if (!document.hidden && this.online) void this.sync(true)
      }, 60 * 60_000)
      if (this.online) void this.sync()
    } catch {
      if (epoch === this.epoch)
        this.patch(
          {
            phase: 'unavailable',
            error: this.message('offlineBrowserUnavailable'),
          },
          'offlineBrowserUnavailable',
        )
    }
  }
  stop() {
    this.epoch++
    clearTimeout(this.timer)
    clearTimeout(this.peerTimer)
    clearInterval(this.refreshTimer)
    this.controller?.abort()
    this.worker?.terminate()
    this.worker = undefined
    for (const pending of this.pending.values())
      pending.reject(new Error(this.message('offlineAccountChanged')))
    this.pending.clear()
    this.events = []
    this.owner = undefined
    this.fileReads.reset()
    offlineAccountRevision.value++
    this.syncing = false
    this.dirty = false
    this.flushing = false
    this.downloading = false
    this.channel?.close()
    this.channel = undefined
    this.patch({
      phase: 'idle',
      messages: 0,
      files: 0,
      bytes: 0,
      since: undefined,
      updatedAt: undefined,
      error: undefined,
    })
    this.invalidateFiles()
  }
  async clear(owner: string) {
    this.revoked.add(owner)
    if ('BroadcastChannel' in window) {
      const channel = new BroadcastChannel('ircord-offline-accounts')
      channel.postMessage({ clear: owner })
      channel.close()
    }
    if (this.owner === owner) this.stop()
    await new Promise<void>((resolve) => {
      if (typeof indexedDB === 'undefined') {
        resolve()
        return
      }
      const deletion = indexedDB.deleteDatabase('ircord-offline-' + owner)
      deletion.onsuccess = deletion.onerror = () => resolve()
    }).catch(() => {})
    if (this.owner && this.owner !== owner) return
    this.patch({
      phase: 'idle',
      messages: 0,
      files: 0,
      bytes: 0,
      since: undefined,
      updatedAt: undefined,
      error: undefined,
    })
    this.invalidateFiles()
  }
  connection(status: string) {
    const was = this.online
    this.online = status === 'connected' && navigator.onLine
    this.patch({ offline: !this.online })
    if (this.online && !was && this.owner) void this.sync()
  }
  event(event: ServerEvent) {
    if (!this.owner || !('seq' in event)) return
    this.events.push(event)
    if (this.events.length > 4096) {
      this.events = []
      if (this.syncing) {
        this.dirty = true
        this.controller?.abort()
      } else void this.sync(true)
      return
    }
    if (event.type === 'history.synced') {
      void this.sync(true)
      return
    }
    if (!this.syncing) {
      clearTimeout(this.timer)
      this.timer = setTimeout(() => void this.flush(), 50)
    }
  }
  private async flush() {
    if (this.flushing || this.syncing || !this.worker || !this.events.length) return
    this.flushing = true
    const rpc = this.session(),
      epoch = this.epoch
    try {
      while (this.events.length && !this.syncing) {
        const batch = this.events.splice(0, 100)
        await rpc('events', batch)
        if (epoch !== this.epoch) return
      }
      this.invalidateFiles()
      await this.refreshInfo(rpc)
      if (epoch !== this.epoch) return
      this.broadcastUpdate()
      void this.downloadFiles()
    } catch {
      if (epoch === this.epoch) {
        this.patch(
          { phase: 'partial', error: this.message('offlineNeedsSync') },
          'offlineNeedsSync',
        )
        if (this.online) void this.sync(true)
      }
    } finally {
      if (epoch === this.epoch) this.flushing = false
    }
  }
  private updateInfo(info: ArchiveInfo) {
    this.patch({
      phase: info.meta?.complete ? 'ready' : info.meta ? 'partial' : 'idle',
      messages: info.messages,
      files: info.files,
      bytes: info.bytes,
      since: info.meta?.since,
      updatedAt: info.meta?.updatedAt,
      error: undefined,
    })
  }
  async refreshInfo(rpc = this.session()) {
    if (!this.worker) return
    const epoch = this.epoch
    const info = await rpc<ArchiveInfo>('status')
    if (epoch === this.epoch) this.updateInfo(info)
  }
  private broadcastUpdate() {
    this.channel?.postMessage({ updated: this.owner })
  }
  async sync(force = false) {
    if (this.syncing) {
      if (force) this.dirty = true
      return
    }
    if (!this.worker || !this.owner || !this.online) return
    this.syncing = true
    const epoch = this.epoch
    const rpc = this.session()
    this.controller?.abort()
    this.controller = new AbortController()
    const signal = this.controller.signal
    this.patch({ phase: 'syncing', error: undefined })
    try {
      const info = await rpc<ArchiveInfo>('status')
      let full = force || !info.meta?.complete
      if (!full && info.meta) {
        const replay = await apiClient().replay(info.meta.state.cursor, signal)
        if (replay.valid) await rpc('events', replay.items)
        else full = true
      }
      if (full) {
        let page = await apiClient().offlineSnapshot(undefined, signal)
        if (!page.state) throw new Error(this.message('offlineMissingState'))
        const generation = crypto.randomUUID(),
          base = page.cursor
        await rpc(
          'begin',
          page.state,
          page.since,
          generation,
          !!info.meta && info.meta.state.cursor > page.cursor,
        )
        while (true) {
          await rpc('page', page.items, page.cursor, generation)
          if (!page.nextCursor) break
          page = await apiClient().offlineSnapshot(page.nextCursor, signal)
        }
        const pinned = await rpc<{ conversationId: string; messageId: string }[]>('pinned')
        for (let offset = 0; offset < pinned.length; offset += 100) {
          const refs = pinned.slice(offset, offset + 100)
          const result = await apiClient().offlinePinned(refs, signal)
          await rpc('reconcilePinned', refs, result.items, result.cursor)
        }
        await rpc('finish', base, generation)
      }
      while (this.events.length) await rpc('events', this.events.splice(0, 100))
      await rpc('maintenance')
      await rpc('retryFiles')
      await this.refreshInfo(rpc)
      if (epoch !== this.epoch) return
      this.invalidateFiles()
      this.broadcastUpdate()
    } catch {
      if (epoch === this.epoch && !signal.aborted)
        this.patch({ phase: 'partial', error: this.message('offlinePartial') }, 'offlinePartial')
    } finally {
      if (epoch === this.epoch) {
        this.syncing = false
        if (this.dirty) {
          this.dirty = false
          void this.sync(true)
        } else {
          if (this.events.length) void this.flush()
          if (this.online) void this.downloadFiles()
        }
      }
    }
  }
  private async downloadFiles() {
    if (this.downloading || !this.online || !this.worker) return
    this.downloading = true
    const epoch = this.epoch
    const rpc = this.session()
    let changed = false
    try {
      while (epoch === this.epoch && this.online) {
        const files = await rpc<OfflineFile[]>('pendingFiles')
        if (!files.length) break
        for (const file of files) {
          if (epoch !== this.epoch || !this.online) break
          try {
            await this.download(file, epoch)
            changed = true
          } catch {
            if (epoch === this.epoch) await rpc('fileFailed', file.url)
          }
        }
      }
      if (changed && epoch === this.epoch) {
        await this.refreshInfo(rpc)
        if (epoch !== this.epoch) return
        this.invalidateFiles()
        this.broadcastUpdate()
      }
    } catch {
      /* A closed or full browser database leaves online use available. */
    } finally {
      if (epoch === this.epoch) this.downloading = false
    }
  }
  private async download(file: OfflineFile, expectedEpoch = this.epoch) {
    if (expectedEpoch !== this.epoch) throw new Error(this.message('offlineAccountChanged'))
    const epoch = this.epoch
    const rpc = this.session()
    const url = new URL(this.apiBase.replace(/\/$/, '') + '/offline/media', location.href)
    url.search = new URLSearchParams({
      conversationId: file.conversationId,
      messageId: file.messageId,
      url: file.url,
    }).toString()
    const response = await fetch(url, { credentials: 'include', signal: this.controller?.signal })
    if (!response.ok) throw new Error(this.message('offlineFileFailed'))
    const blob = await response.blob()
    if (epoch === this.epoch) {
      await rpc('saveFile', file.url, blob)
      if (epoch === this.epoch) this.invalidateFiles(file.url)
    }
  }
  async file(url: string, signal?: AbortSignal): Promise<FileInfo | undefined> {
    if (!this.owner) return
    const epoch = this.epoch
    const file = await this.fileReads.read(url, signal)
    if (epoch !== this.epoch) throw new Error(this.message('offlineAccountChanged'))
    return file
  }
  async pin(message: Message, value: boolean) {
    if (!this.owner) throw new Error(this.message('offlineUnavailable'))
    const rpc = this.session(),
      epoch = this.epoch
    const info = await rpc<ArchiveInfo>('status')
    await rpc(
      'page',
      [message],
      message.liveSeq ?? info.meta?.state.cursor ?? 0,
      info.meta?.generation ?? 'pinned',
      value,
    )
    await rpc('pin', message.id, value)
    if (value) {
      void navigator.storage?.persist?.().catch(() => {})
      if (this.online)
        for (const file of (await rpc<OfflineFile[]>('pendingFiles')).filter(
          (file) => file.messageId === message.id,
        ))
          await this.download(file, epoch)
    }
    await this.refreshInfo(rpc)
    if (epoch !== this.epoch) return
    this.invalidateFiles()
    this.broadcastUpdate()
  }
  async history(
    id: string,
    query: { before?: string; around?: string; limit?: number },
  ): Promise<MessagePage> {
    return this.rpc('history', id, query.before, query.around, query.limit ?? 40)
  }
  async search(text: string, filters: SearchFilters, cursor?: string): Promise<LocalSearchPage> {
    return this.rpc('search', text, filters, cursor, 50)
  }
  available() {
    return !!this.owner && !!this.worker
  }
}
export const offlineArchive = new OfflineArchive()
// A replaced development module must release its worker and recurring sync timer.
if (import.meta.hot) import.meta.hot.dispose(() => offlineArchive.stop())
const unreachable = (error: unknown) => {
  const code = errorStatus(error)
  return !code || code >= 500
}

export async function readMessages(
  id: string,
  query: { before?: string; around?: string; limit?: number } = {},
  signal?: AbortSignal,
): Promise<MessagePage> {
  // A reconnecting live socket is not proof that HTTP history is unavailable. Online reads
  // always try the server, including a window whose previous page came from IndexedDB.
  if (offlineArchive.available() && !navigator.onLine) return offlineArchive.history(id, query)
  try {
    const boundary = boundaryMessage(query.before)
    let online = query
    if (query.before?.startsWith('message:') || query.before?.startsWith('offline:')) {
      if (!boundary) throw new Error('Invalid history boundary')
      const around = await apiClient().messages(id, { around: boundary, limit: 1 }, signal)
      if (!around.nextCursor) return { ...around, items: [], hasNewer: false, nextCursor: null }
      online = { ...query, before: around.nextCursor }
    }
    return await apiClient().messages(id, online, signal)
  } catch (error) {
    if (signal?.aborted || !unreachable(error) || !offlineArchive.available()) throw error
    return offlineArchive.history(id, query)
  }
}
export async function readSearch(
  text: string,
  filters: SearchFilters,
  cursor?: string,
  signal?: AbortSignal,
): Promise<LocalSearchPage> {
  if (
    offlineArchive.available() &&
    (offlineStatus.value.offline || cursor?.startsWith('offline-search:'))
  )
    return offlineArchive.search(text, filters, cursor)
  try {
    return await apiClient().search({ ...searchParams(text, filters), cursor }, signal)
  } catch (error) {
    if (signal?.aborted || !unreachable(error) || !offlineArchive.available()) throw error
    return offlineArchive.search(text, filters, cursor)
  }
}
