/// <reference lib="webworker" />
import type {
  Bootstrap,
  Message,
  MessagePage,
  SearchResult,
  ServerEvent,
} from '../../shared/types/chat'
import {
  archiveState,
  messageFiles,
  OFFLINE_DB_PREFIX,
  OFFLINE_FILE_BUDGET,
  OFFLINE_WINDOW,
  type OfflineFile,
  type PartyFences,
} from '../../shared/offline'
import { compareMessages } from '../../shared/utils/message-page'
import { boundaryMessage } from '../../shared/utils/history-cursor'
import { matchesSearch, searchTokens, type SearchFilters } from '../../shared/utils/search-filter'

interface Row {
  id: string
  conversationId: string
  at: number
  position: number
  seq: number
  generation: string
  tokens: string[]
  message: Message
  pinned: boolean
}
interface FileRow {
  url: string
  name: string
  automatic: boolean
  at: number
  refs: { id: string; conversationId: string; pinned: boolean }[]
  messageIds: string[]
  blob?: Blob
  failed?: boolean
}
interface Meta {
  state: Bootstrap
  since: string
  complete: boolean
  generation: string
  updatedAt?: string
  partyFences?: PartyFences
}
let db: IDBDatabase | undefined
let account = ''
let revoked = false
let queue = Promise.resolve()
const request = <T>(value: IDBRequest<T>) =>
  new Promise<T>((resolve, reject) => {
    value.onsuccess = () => resolve(value.result)
    value.onerror = () => reject(value.error)
  })
const done = (tx: IDBTransaction) => {
  const completion = new Promise<void>((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onabort = tx.onerror = () =>
      reject(
        tx.error ??
          new OfflineWorkerError('offlineTransactionFailed', 'Offline transaction failed'),
      )
  })
  // A failed request can reject before its caller reaches the completion await.
  void completion.catch(() => {})
  return completion
}
const encodeCursor = (value: unknown) =>
  btoa(String.fromCharCode(...new TextEncoder().encode(JSON.stringify(value))))
const decodeCursor = (value: string) =>
  JSON.parse(
    new TextDecoder().decode(
      Uint8Array.from(atob(value.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0)),
    ),
  )
class OfflineWorkerError extends Error {
  constructor(
    public readonly key: string,
    message: string,
  ) {
    super(message)
  }
}
function database() {
  if (!db || revoked) throw new OfflineWorkerError('offlineClosed', 'Offline storage is closed')
  return db
}
async function open(owner: string) {
  if (revoked) throw new OfflineWorkerError('offlineSignedOut', 'Offline account was signed out')
  if (db && account === owner) return
  db?.close()
  const opening = indexedDB.open(OFFLINE_DB_PREFIX + owner, 1)
  opening.onupgradeneeded = () => {
    const value = opening.result
    value.createObjectStore('meta')
    const messages = value.createObjectStore('messages', { keyPath: 'id' })
    messages.createIndex('conversation', ['conversationId', 'at', 'position', 'id'])
    messages.createIndex('at', 'at')
    messages.createIndex('tokens', 'tokens', { multiEntry: true })
    value
      .createObjectStore('files', { keyPath: 'url' })
      .createIndex('messages', 'messageIds', { multiEntry: true })
  }
  db = await request(opening)
  account = owner
  db.onversionchange = () => {
    revoked = true
    db?.close()
    db = undefined
  }
}
async function readMeta(): Promise<Meta | undefined> {
  return request(database().transaction('meta').objectStore('meta').get('archive'))
}
async function clearFileRef(files: IDBObjectStore, id: string) {
  const all: FileRow[] = await request(files.index('messages').getAll(id))
  for (const file of all)
    if (file.refs.some((ref) => ref.id === id)) {
      file.refs = file.refs.filter((ref) => ref.id !== id)
      file.messageIds = file.refs.map((ref) => ref.id)
      if (file.refs.length) files.put(file)
      else files.delete(file.url)
    }
}
async function putMessage(
  tx: IDBTransaction,
  message: Message,
  seq: number,
  generation: string,
  forcePin = false,
) {
  const messages = tx.objectStore('messages')
  const previous: Row | undefined = await request(messages.get(message.id))
  if (previous && previous.seq > seq) {
    messages.put({ ...previous, generation })
    return
  }
  const at = Date.parse(message.createdAt)
  if (at < Date.now() - OFFLINE_WINDOW && !previous?.pinned && !forcePin) return
  const files = tx.objectStore('files')
  const urls = new Set(messageFiles(message).map((file) => file.url))
  if (previous && messageFiles(previous.message).some((file) => !urls.has(file.url)))
    await clearFileRef(files, message.id)
  const pinned = (previous?.pinned || forcePin) && !message.deleted
  messages.put({
    id: message.id,
    conversationId: message.conversationId,
    at,
    position: message.position ?? 0,
    seq,
    generation,
    tokens: message.deleted ? [] : [...new Set(searchTokens(message.text))],
    message,
    pinned,
  } satisfies Row)
  if (message.deleted) {
    await clearFileRef(files, message.id)
    return
  }
  for (const candidate of messageFiles(message)) {
    const previousFile: FileRow | undefined = await request(files.get(candidate.url))
    const refs = (previousFile?.refs ?? []).filter((ref) => ref.id !== message.id)
    refs.push({ id: message.id, conversationId: message.conversationId, pinned })
    files.put({
      ...previousFile,
      url: candidate.url,
      name: candidate.name,
      automatic: candidate.automatic,
      at,
      refs,
      messageIds: refs.map((ref) => ref.id),
    } satisfies FileRow)
  }
}
async function prune(tx: IDBTransaction, meta: Meta, replace = false, base = 0) {
  const messages = tx.objectStore('messages')
  const rows: Row[] = await request(messages.getAll())
  const allowed = new Set(meta.state.conversations.map((c) => c.id))
  for (const row of rows)
    if (
      !allowed.has(row.conversationId) ||
      (!row.pinned &&
        (row.at < Date.now() - OFFLINE_WINDOW ||
          (replace && row.generation !== meta.generation && row.seq <= base)))
    ) {
      messages.delete(row.id)
      await clearFileRef(tx.objectStore('files'), row.id)
    }
}
async function writePage(items: Message[], cursor: number, generation: string, forcePin = false) {
  const tx = database().transaction(['meta', 'messages', 'files'], 'readwrite')
  const completion = done(tx)
  const meta: Meta | undefined = await request(tx.objectStore('meta').get('archive'))
  if (meta && meta.generation !== generation) {
    await completion
    throw new OfflineWorkerError('offlineSuperseded', 'Offline snapshot was superseded')
  }
  const allowed = new Set(meta?.state.conversations.map((c) => c.id) ?? [])
  for (const message of items)
    if (allowed.has(message.conversationId))
      await putMessage(tx, message, cursor, generation, forcePin)
  await completion
}
async function begin(state: Bootstrap, since: string, generation: string, reset = false) {
  const tx = database().transaction(['meta', 'messages', 'files'], 'readwrite')
  const completion = done(tx)
  const previous: Meta | undefined = await request(tx.objectStore('meta').get('archive'))
  if (reset) {
    tx.objectStore('messages').clear()
    tx.objectStore('files').clear()
  }
  const meta: Meta = {
    state: !reset && previous && previous.state.cursor > state.cursor ? previous.state : state,
    since,
    generation,
    complete: false,
    partyFences: previous?.partyFences ?? {},
  }
  meta.state.conversations = meta.state.conversations
    .map((c) => {
      if (!meta.partyFences?.[c.id]?.converted || !c.party) return c
      return { ...c, party: undefined }
    })
    .filter((c) => {
      const fence = meta.partyFences?.[c.id]
      return (
        !fence ||
        (!!fence.converted && !c.party) ||
        (!!c.party &&
          (c.party.revision > fence.revision ||
            (!fence.removed && c.party.revision === fence.revision)))
      )
    })
  tx.objectStore('meta').put(meta, 'archive')
  await prune(tx, meta)
  await completion
}
async function finish(base: number, generation: string) {
  const tx = database().transaction(['meta', 'messages', 'files'], 'readwrite')
  const completion = done(tx)
  const meta: Meta = await request(tx.objectStore('meta').get('archive'))
  if (meta.generation !== generation) {
    await completion
    throw new OfflineWorkerError('offlineSuperseded', 'Offline snapshot was superseded')
  }
  await prune(tx, meta, true, base)
  meta.complete = true
  meta.updatedAt = new Date().toISOString()
  tx.objectStore('meta').put(meta, 'archive')
  await completion
}
async function events(batch: ServerEvent[]) {
  const tx = database().transaction(['meta', 'messages', 'files'], 'readwrite')
  const completion = done(tx)
  const meta: Meta | undefined = await request(tx.objectStore('meta').get('archive'))
  if (!meta) {
    await completion
    return
  }
  try {
    for (const event of batch) {
      if (!('seq' in event) || event.seq <= meta.state.cursor) continue
      meta.partyFences ??= {}
      meta.state = archiveState(meta.state, event, meta.partyFences)
      if (
        (event.type === 'message.created' || event.type === 'message.updated') &&
        meta.state.conversations.some((c) => c.id === event.data.conversationId)
      )
        await putMessage(tx, event.data, event.seq, meta.generation)
      if (
        event.type === 'conversation.removed' ||
        event.type === 'network.removed' ||
        event.type === 'party.removed'
      )
        await prune(tx, meta)
    }
    meta.updatedAt = new Date().toISOString()
    tx.objectStore('meta').put(meta, 'archive')
  } catch (error) {
    tx.abort()
    await completion.catch(() => {})
    throw error
  }
  await completion
}
async function status() {
  const meta = await readMeta()
  const tx = database().transaction(['messages', 'files'])
  const messages = await request(tx.objectStore('messages').count())
  const files: FileRow[] = await request(tx.objectStore('files').getAll())
  return {
    meta,
    messages,
    files: files.filter((f) => f.blob).length,
    bytes: files.reduce((n, f) => n + (f.blob?.size ?? 0), 0),
  }
}
async function history(
  id: string,
  before: string | undefined,
  around: string | undefined,
  limit: number,
): Promise<MessagePage> {
  const meta = await readMeta()
  if (!meta?.state.conversations.some((c) => c.id === id))
    throw new OfflineWorkerError(
      'offlineConversationUnavailable',
      'Conversation is not available offline',
    )
  const tx = database().transaction('messages')
  const range = IDBKeyRange.bound(
    [id, -Number.MAX_SAFE_INTEGER, -Number.MAX_SAFE_INTEGER, ''],
    [id, Number.MAX_SAFE_INTEGER, Number.MAX_SAFE_INTEGER, '\uffff'],
  )
  const rows: Row[] = await request(tx.objectStore('messages').index('conversation').getAll(range))
  const items = rows.map((row) => ({ ...row.message, liveSeq: row.seq })).sort(compareMessages)
  let end = items.length
  if (before) {
    const boundary = boundaryMessage(before) ?? decodeCursor(before).id
    const found = items.findIndex((m) => m.id === boundary)
    if (found < 0) {
      throw new OfflineWorkerError('offlineHistoryExpired', 'Offline history cursor expired')
    }
    end = found
  }
  let start = Math.max(0, end - limit)
  let hasNewer = false
  if (around) {
    const at = items.findIndex((m) => m.id === around)
    if (at < 0)
      throw new OfflineWorkerError('offlineMessageUnavailable', 'Message is not available offline')
    start = Math.max(0, at - Math.floor((limit - 1) / 2))
    end = Math.min(items.length, start + limit)
    hasNewer = end < items.length
  }
  return {
    items: items.slice(start, end),
    cursor: meta.state.cursor,
    nextCursor: start > 0 ? 'offline:' + encodeCursor({ id: items[start]!.id }) : null,
    hasNewer,
    offline: true,
  }
}
async function search(text: string, filters: SearchFilters, cursor?: string, limit = 50) {
  const meta = await readMeta()
  if (!meta) return { items: [], nextCursor: null, offline: true }
  const members = new Map(meta.state.members.map((m) => [m.id, m]))
  const conversations = new Map(meta.state.conversations.map((c) => [c.id, c]))
  if (filters.in.some((id) => !conversations.has(id)))
    throw new OfflineWorkerError(
      'offlineConversationUnavailable',
      'Conversation is not available offline',
    )
  const terms = searchTokens(text)
  const store = database().transaction('messages').objectStore('messages')
  const rows: Row[] = terms.length
    ? await request(
        store
          .index('tokens')
          .getAll(
            terms.length > 1
              ? IDBKeyRange.only(terms[0])
              : IDBKeyRange.bound(terms[0]!, terms[0]! + '\uffff'),
          ),
      )
    : await request(store.getAll())
  const signature = JSON.stringify([text, filters])
  let boundary = cursor ? decodeCursor(cursor.replace(/^offline-search:/, '')) : undefined
  if (boundary && cursor?.startsWith('offline-search:') && boundary.q !== signature)
    throw new OfflineWorkerError('offlineSearchExpired', 'Offline search cursor expired')
  if (boundary && !cursor?.startsWith('offline-search:')) {
    // A network failure while paging can leave a server cursor in the UI.
    // Continue from that saved message using the archive's own ordering.
    const row = rows.find((row) => row.id === boundary.id)
    if (!row)
      return {
        items: [],
        nextCursor: null,
        offline: true,
        since: meta.since,
        updatedAt: meta.updatedAt,
      }
    boundary = { q: signature, at: row.at, id: row.id }
  }
  const items: SearchResult[] = []
  for (const row of [...new Map(rows.map((row) => [row.id, row])).values()].sort(
    (a, b) => b.at - a.at || (a.id < b.id ? 1 : -1),
  )) {
    const message = row.message
    if (boundary && (row.at > boundary.at || (row.at === boundary.at && row.id >= boundary.id)))
      continue
    if (
      !conversations.has(row.conversationId) ||
      message.deleted ||
      !['message', 'action', 'notice'].includes(message.kind) ||
      !matchesSearch(message, text, filters, (id) => members.get(id))
    )
      continue
    const conversation = conversations.get(row.conversationId)!
    items.push({
      message,
      conversation: {
        ...conversation,
        joined: conversation.joined ?? false,
        memberModes: conversation.memberModes ?? {},
        modes: conversation.modes ?? '',
      },
    })
    if (items.length > limit) break
  }
  let nextCursor: string | null = null
  if (items.length > limit) {
    items.pop()
    const last = items.at(-1)!.message
    nextCursor =
      'offline-search:' +
      encodeCursor({ q: signature, at: Date.parse(last.createdAt), id: last.id })
  }
  return { items, nextCursor, offline: true, since: meta.since, updatedAt: meta.updatedAt }
}
async function pendingFiles(): Promise<OfflineFile[]> {
  const files: FileRow[] = await request(
    database().transaction('files').objectStore('files').getAll(),
  )
  return files
    .filter(
      (file) =>
        !file.blob && !file.failed && (file.automatic || file.refs.some((ref) => ref.pinned)),
    )
    .map((file) => ({
      url: file.url,
      name: file.name,
      automatic: file.automatic,
      messageId: file.refs[0]!.id,
      conversationId: file.refs[0]!.conversationId,
    }))
}
async function saveFile(url: string, blob: Blob) {
  const tx = database().transaction('files', 'readwrite'),
    completion = done(tx),
    store = tx.objectStore('files')
  const file: FileRow | undefined = await request(store.get(url))
  if (file) {
    const all: FileRow[] = await request(store.getAll())
    let bytes =
      all.reduce((n, f) => n + (f.blob?.size ?? 0), 0) - (file.blob?.size ?? 0) + blob.size
    for (const old of all.sort((a, b) => a.at - b.at))
      if (
        bytes > OFFLINE_FILE_BUDGET &&
        old.url !== url &&
        !old.refs.some((ref) => ref.pinned) &&
        old.blob
      ) {
        bytes -= old.blob.size
        delete old.blob
        old.failed = true
        store.put(old)
      }
    file.blob = blob
    file.failed = false
    store.put(file)
  }
  await completion
}
async function pin(id: string, value: boolean) {
  const tx = database().transaction(['messages', 'files'], 'readwrite'),
    completion = done(tx)
  const row: Row | undefined = await request(tx.objectStore('messages').get(id))
  if (!row || row.message.deleted)
    throw new OfflineWorkerError('offlineMessageUnavailable', 'Message is not available offline')
  row.pinned = value
  tx.objectStore('messages').put(row)
  const files: FileRow[] = await request(tx.objectStore('files').getAll())
  for (const file of files)
    if (file.refs.some((ref) => ref.id === id)) {
      file.refs = file.refs.map((ref) => (ref.id === id ? { ...ref, pinned: value } : ref))
      file.failed = false
      tx.objectStore('files').put(file)
    }
  await completion
}
async function pinned() {
  const rows: Row[] = await request(
    database().transaction('messages').objectStore('messages').getAll(),
  )
  return rows
    .filter((row) => row.pinned)
    .map((row) => ({ conversationId: row.conversationId, messageId: row.id }))
}
async function reconcilePinned(refs: { messageId: string }[], items: Message[], cursor: number) {
  const meta = await readMeta()
  await writePage(items, cursor, meta?.generation ?? 'pinned')
  const missing = refs.filter((ref) => !items.some((item) => item.id === ref.messageId))
  const tx = database().transaction(['messages', 'files'], 'readwrite'),
    completion = done(tx)
  for (const ref of missing) {
    const row: Row | undefined = await request(tx.objectStore('messages').get(ref.messageId))
    if (row && row.seq <= cursor) {
      tx.objectStore('messages').delete(ref.messageId)
      await clearFileRef(tx.objectStore('files'), ref.messageId)
    }
  }
  await completion
}
// Arguments arrive structured-cloned from the typed archive client; each case forwards them positionally.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function handle(method: string, args: any[]) {
  switch (method) {
    case 'open':
      await open(args[0])
      return status()
    case 'status':
      return status()
    case 'begin':
      return begin(args[0], args[1], args[2], args[3])
    case 'page':
      return writePage(args[0], args[1], args[2], args[3])
    case 'finish':
      return finish(args[0], args[1])
    case 'events':
      return events(args[0])
    case 'history':
      return history(args[0], args[1], args[2], args[3])
    case 'search':
      return search(args[0], args[1], args[2], args[3])
    case 'pendingFiles':
      return pendingFiles()
    case 'saveFile':
      return saveFile(args[0], args[1])
    case 'file':
      return request(database().transaction('files').objectStore('files').get(args[0]))
    case 'pin':
      return pin(args[0], args[1])
    case 'pinned':
      return pinned()
    case 'reconcilePinned':
      return reconcilePinned(args[0], args[1], args[2])
    case 'fileFailed': {
      const tx = database().transaction('files', 'readwrite'),
        completion = done(tx),
        store = tx.objectStore('files')
      const file = await request(store.get(args[0]))
      if (file) store.put({ ...file, failed: true })
      await completion
      return
    }
    case 'retryFiles': {
      const tx = database().transaction('files', 'readwrite'),
        completion = done(tx),
        store = tx.objectStore('files')
      for (const file of await request(store.getAll())) store.put({ ...file, failed: false })
      await completion
      return
    }
    case 'maintenance': {
      const tx = database().transaction(['meta', 'messages', 'files'], 'readwrite'),
        completion = done(tx)
      const meta: Meta | undefined = await request(tx.objectStore('meta').get('archive'))
      if (meta) {
        meta.since = new Date(
          Math.max(Date.parse(meta.since), Date.now() - OFFLINE_WINDOW),
        ).toISOString()
        await prune(tx, meta)
        tx.objectStore('meta').put(meta, 'archive')
      }
      await completion
      return
    }
    case 'clear':
      revoked = true
      db?.close()
      db = undefined
      await request(indexedDB.deleteDatabase(OFFLINE_DB_PREFIX + account))
      return
    default:
      throw new OfflineWorkerError('offlineOperationFailed', 'Unknown offline operation')
  }
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
self.onmessage = ({ data }: { data: { id: number; method: string; args: any[] } }) => {
  queue = queue.then(async () => {
    try {
      const result = await handle(data.method, data.args)
      self.postMessage({ id: data.id, result })
    } catch (error) {
      self.postMessage({
        id: data.id,
        error: error instanceof Error ? error.message : 'Offline storage failed',
        errorKey:
          error instanceof OfflineWorkerError
            ? error.key
            : error instanceof Error
              ? undefined
              : 'offlineOperationFailed',
      })
    }
  })
}
