import type { Upload } from '../../shared/types/chat'

export interface PendingFile {
  key: string
  file: File
  /** Object URL for image previews. */
  preview?: string
  width?: number
  height?: number
  progress: number
  status: 'ready' | 'uploading' | 'done' | 'failed'
  error?: string
  upload?: Upload
}

const queues = reactive<Record<string, PendingFile[]>>({})
const MAX_FILES = 10

/** Matches `image/*` style patterns from `capabilities.upload.types`. */
const accepts = (types: string[], type: string) =>
  types.some((pattern) =>
    pattern.endsWith('/*') ? type.startsWith(pattern.slice(0, -1)) : type === pattern,
  )

function measure(file: File): Promise<{ width?: number; height?: number }> {
  if (!file.type.startsWith('image/')) return Promise.resolve({})
  return createImageBitmap(file)
    .then((bitmap) => {
      const size = { width: bitmap.width, height: bitmap.height }
      bitmap.close()
      return size
    })
    .catch(() => ({}))
}

/**
 * Files waiting to be sent in each conversation. Uploading happens on send; each file then goes
 * out as its own message whose text is the file's URL, so other IRC clients see a link.
 */
export function useUploads() {
  const chat = useChatStore()
  const capability = computed(() => chat.data?.capabilities?.upload)
  const pending = computed(() => queues[chat.activeId] ?? [])

  async function add(files: FileList | File[]) {
    const limits = capability.value
    if (!limits) return chat.notify('This server doesn’t accept file uploads.')
    // Read back through the reactive map so pushes are tracked (`??=` returns the raw array).
    if (!queues[chat.activeId]) queues[chat.activeId] = []
    const list = queues[chat.activeId]!
    for (const file of [...files]) {
      if (list.length >= MAX_FILES)
        return chat.notify(`You can attach up to ${MAX_FILES} files at a time.`)
      if (!accepts(limits.types, file.type)) {
        chat.notify(`${file.name} isn’t a supported file type.`)
        continue
      }
      if (file.size > limits.maxBytes) {
        chat.notify(`${file.name} is larger than ${formatBytes(limits.maxBytes)}.`)
        continue
      }
      list.push({
        key: crypto.randomUUID(),
        file,
        preview: file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined,
        ...(await measure(file)),
        progress: 0,
        status: 'ready',
      })
    }
  }
  function remove(key: string, conversationId = chat.activeId) {
    const list = queues[conversationId]
    const item = list?.find((p) => p.key === key)
    if (item?.preview) URL.revokeObjectURL(item.preview)
    if (list) queues[conversationId] = list.filter((p) => p.key !== key)
  }

  function upload(item: PendingFile, networkId: string): Promise<Upload> {
    return apiClient().upload(item.file, networkId, progress => { item.progress = progress })
  }

  /** Uploads everything pending here and sends one message per file. */
  async function sendAll() {
    const conversationId = chat.activeId
    for (const item of [...(queues[conversationId] ?? [])]) {
      if (item.status === 'uploading' || item.status === 'done') continue
      item.status = 'uploading'
      item.error = undefined
      try {
        const networkId = chat.data?.conversations.find((c) => c.id === conversationId)?.networkId
        if (!networkId) throw new Error('The conversation is no longer available.')
        item.upload ??= await upload(item, networkId)
        const url = new URL(item.upload.url, location.origin).href
        if (
          chat.activeId !== conversationId ||
          !chat.send(url, undefined, undefined, { upload: { ...item.upload, url } })
        ) {
          item.status = 'ready'
          return
        }
        item.status = 'done'
        remove(item.key, conversationId)
      } catch (error) {
        item.status = 'failed'
        item.error = error instanceof Error ? error.message : 'The upload failed.'
      }
    }
  }

  return { capability, pending, add, remove, sendAll }
}

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(bytes < 10 * 1024 * 1024 ? 1 : 0)} MB`
}
