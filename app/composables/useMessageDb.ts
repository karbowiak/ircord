import type { Message } from '~/types'

// --- Types ---

export type StoredMessage = Message & {
  serverId: string
  ircMsgid: string | null
}

type ChannelMeta = {
  channelId: string
  lastMsgid: string | null
  lastTimestamp: string | null
}

// --- Constants ---

const DB_NAME = 'ircord-messages'
const DB_VERSION = 1
const MESSAGES_STORE = 'messages'
const META_STORE = 'meta'

// --- Singleton ---

let dbInstance: IDBDatabase | null = null

function openDb(): Promise<IDBDatabase> {
  if (typeof indexedDB === 'undefined') {
    return Promise.reject(new Error('IndexedDB not available (SSR)'))
  }
  if (dbInstance) return Promise.resolve(dbInstance)

  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)

    request.onupgradeneeded = () => {
      const db = request.result

      if (!db.objectStoreNames.contains(MESSAGES_STORE)) {
        const msgStore = db.createObjectStore(MESSAGES_STORE, { keyPath: 'id' })
        msgStore.createIndex('channelId', 'channelId', { unique: false })
        msgStore.createIndex('ircMsgid', 'ircMsgid', { unique: false })
        msgStore.createIndex('channelId_timestamp', ['channelId', 'timestamp'], { unique: false })
      }

      if (!db.objectStoreNames.contains(META_STORE)) {
        db.createObjectStore(META_STORE, { keyPath: 'channelId' })
      }
    }

    request.onsuccess = () => {
      dbInstance = request.result
      resolve(dbInstance)
    }

    request.onerror = () => {
      reject(request.error)
    }
  })
}

// --- Helpers ---

function txStore(db: IDBDatabase, store: string, mode: IDBTransactionMode): IDBObjectStore {
  return db.transaction(store, mode).objectStore(store)
}

function promisify<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

function promisifyTransaction(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
}

// --- Public API ---

export function useMessageDb() {
  async function putMessages(msgs: StoredMessage[]): Promise<void> {
    if (!msgs.length) return
    const db = await openDb()
    const tx = db.transaction(MESSAGES_STORE, 'readwrite')
    const store = tx.objectStore(MESSAGES_STORE)
    for (const msg of msgs) {
      store.put(msg)
    }
    await promisifyTransaction(tx)
  }

  async function putMessage(msg: StoredMessage): Promise<void> {
    const db = await openDb()
    await promisify(txStore(db, MESSAGES_STORE, 'readwrite').put(msg))
  }

  async function getChannelMessages(channelId: string, limit = 200): Promise<StoredMessage[]> {
    const db = await openDb()
    const store = txStore(db, MESSAGES_STORE, 'readonly')
    const index = store.index('channelId_timestamp')

    // IDBKeyRange for a specific channelId, ordered by timestamp
    const range = IDBKeyRange.bound([channelId, ''], [channelId, '\uffff'])

    return new Promise((resolve, reject) => {
      const results: StoredMessage[] = []
      const cursorReq = index.openCursor(range, 'prev') // newest first

      cursorReq.onsuccess = () => {
        const cursor = cursorReq.result
        if (cursor && results.length < limit) {
          results.push(cursor.value)
          cursor.continue()
        } else {
          resolve(results.reverse()) // return in chronological order
        }
      }

      cursorReq.onerror = () => reject(cursorReq.error)
    })
  }

  async function hasMessageByMsgid(msgid: string): Promise<boolean> {
    if (!msgid) return false
    const db = await openDb()
    const store = txStore(db, MESSAGES_STORE, 'readonly')
    const index = store.index('ircMsgid')
    const count = await promisify(index.count(msgid))
    return count > 0
  }

  async function getLastMessageMeta(channelId: string): Promise<ChannelMeta | null> {
    const db = await openDb()
    const result = await promisify(txStore(db, META_STORE, 'readonly').get(channelId))
    return result || null
  }

  async function setLastMessageMeta(channelId: string, meta: { msgid: string | null; timestamp: string | null }): Promise<void> {
    const db = await openDb()
    await promisify(txStore(db, META_STORE, 'readwrite').put({
      channelId,
      lastMsgid: meta.msgid,
      lastTimestamp: meta.timestamp,
    }))
  }

  async function clearChannel(channelId: string): Promise<void> {
    const db = await openDb()
    const store = txStore(db, MESSAGES_STORE, 'readwrite')
    const index = store.index('channelId')
    const keys = await promisify(index.getAllKeys(channelId))
    for (const key of keys) {
      store.delete(key)
    }
  }

  async function searchMessages(query: string, limit = 50): Promise<StoredMessage[]> {
    if (!query.trim()) return []
    const db = await openDb()
    const lowerQuery = query.toLowerCase()

    return new Promise((resolve, reject) => {
      const results: StoredMessage[] = []
      const store = txStore(db, MESSAGES_STORE, 'readonly')
      const cursorReq = store.openCursor()

      cursorReq.onsuccess = () => {
        const cursor = cursorReq.result
        if (!cursor || results.length >= limit) {
          resolve(results)
          return
        }

        const msg: StoredMessage = cursor.value
        if (msg.content.toLowerCase().includes(lowerQuery)) {
          results.push(msg)
        }
        cursor.continue()
      }

      cursorReq.onerror = () => reject(cursorReq.error)
    })
  }

  return {
    putMessages,
    putMessage,
    getChannelMessages,
    hasMessageByMsgid,
    getLastMessageMeta,
    setLastMessageMeta,
    clearChannel,
    searchMessages,
  }
}
