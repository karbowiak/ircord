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

export const offlineStatus = shallowRef<OfflineStatus>({
  phase: 'idle',
  offline: false,
  messages: 0,
  files: 0,
  bytes: 0,
})
export const offlineRevision = shallowRef(0)
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

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private rpc<T = any>(method: string, ...args: unknown[]): Promise<T> {
    if (!this.worker) return Promise.reject(new Error('Offline storage is unavailable'))
    const id = ++this.serial
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject })
      try {
        this.worker!.postMessage({ id, method, args: args.map((value) => toRaw(value)) })
      } catch (error) {
        this.pending.delete(id)
        reject(error instanceof Error ? error : new Error('Offline request could not be sent'))
      }
    })
  }
  private patch(values: Partial<OfflineStatus>) {
    offlineStatus.value = { ...offlineStatus.value, ...values }
  }
  // A task keeps its worker identity across every await. An old download or
  // snapshot must never write into a newly signed-in account's database.
  private session(): RPC {
    const worker = this.worker
    return (method, ...args) =>
      worker && worker === this.worker
        ? this.rpc(method, ...args)
        : Promise.reject(new Error('Offline account changed'))
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
        const pending = this.pending.get(data.id)
        if (!pending) return
        this.pending.delete(data.id)
        if (data.error) pending.reject(new Error(data.error))
        else pending.resolve(data.result)
      }
      this.worker.onerror = () => {
        this.stop()
        this.patch({ phase: 'unavailable', error: 'Offline storage could not start.' })
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
            this.peerTimer = setTimeout(() => {
              void this.refreshInfo(rpc)
                .then(() => offlineRevision.value++)
                .catch(() => {})
            }, 100)
          }
        }
      }
      const info = await this.rpc<ArchiveInfo>('open', owner)
      if (epoch !== this.epoch) return
      this.updateInfo(info)
      this.refreshTimer = setInterval(() => {
        if (!document.hidden && this.online) void this.sync(true)
      }, 60 * 60_000)
      if (this.online) void this.sync()
    } catch {
      if (epoch === this.epoch)
        this.patch({
          phase: 'unavailable',
          error: 'Offline storage is unavailable in this browser.',
        })
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
      pending.reject(new Error('Offline account changed'))
    this.pending.clear()
    this.events = []
    this.owner = undefined
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
    offlineRevision.value++
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
    offlineRevision.value++
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
      }
      offlineRevision.value++
      await this.refreshInfo(rpc)
      this.broadcastUpdate()
      void this.downloadFiles()
    } catch {
      if (epoch === this.epoch) {
        this.patch({ phase: 'partial', error: 'Offline history needs to synchronize again.' })
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
    if (this.worker) this.updateInfo(await rpc<ArchiveInfo>('status'))
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
        if (!page.state) throw new Error('Offline snapshot is missing state')
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
      offlineRevision.value++
      this.broadcastUpdate()
    } catch {
      if (epoch === this.epoch && !signal.aborted)
        this.patch({ phase: 'partial', error: 'Some history is not available offline yet.' })
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
    try {
      while (epoch === this.epoch && this.online) {
        const files = await rpc<OfflineFile[]>('pendingFiles')
        if (!files.length) break
        for (const file of files) {
          if (epoch !== this.epoch || !this.online) break
          try {
            await this.download(file, epoch)
          } catch {
            if (epoch === this.epoch) await rpc('fileFailed', file.url)
          }
        }
      }
      if (epoch === this.epoch) {
        await this.refreshInfo(rpc)
        offlineRevision.value++
        this.broadcastUpdate()
      }
    } catch {
      /* A closed or full browser database leaves online use available. */
    } finally {
      if (epoch === this.epoch) this.downloading = false
    }
  }
  private async download(file: OfflineFile, expectedEpoch = this.epoch) {
    if (expectedEpoch !== this.epoch) throw new Error('Offline account changed')
    const epoch = this.epoch
    const rpc = this.session()
    const url = new URL(this.apiBase.replace(/\/$/, '') + '/offline/media', location.href)
    url.search = new URLSearchParams({
      conversationId: file.conversationId,
      messageId: file.messageId,
      url: file.url,
    }).toString()
    const response = await fetch(url, { credentials: 'include', signal: this.controller?.signal })
    if (!response.ok) throw new Error('This file could not be saved offline.')
    const blob = await response.blob()
    if (epoch === this.epoch) {
      await rpc('saveFile', file.url, blob)
      offlineRevision.value++
    }
  }
  async file(url: string): Promise<FileInfo | undefined> {
    return this.owner ? this.rpc('file', url) : undefined
  }
  async pin(message: Message, value: boolean) {
    if (!this.owner) throw new Error('Offline storage is unavailable')
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
    offlineRevision.value++
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
const unreachable = (error: unknown) => {
  const code = errorStatus(error)
  return !code || code >= 500
}

export async function readMessages(
  id: string,
  query: { before?: string; around?: string; limit?: number } = {},
  signal?: AbortSignal,
): Promise<MessagePage> {
  if (
    offlineArchive.available() &&
    (offlineStatus.value.offline || query.before?.startsWith('offline:'))
  )
    return offlineArchive.history(id, query)
  try {
    return await apiClient().messages(id, query, signal)
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
