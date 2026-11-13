import type { Message, Upload } from '../../../shared/types/chat'
import { UploadError } from '../../../shared/api/client'
import { channelTarget } from '../../../shared/utils/irc'
import { formatBytes } from '../../../shared/utils/bytes'

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
  errorKey?: string
  upload?: Upload
  /** Stops the request while this file uploads. */
  controller?: AbortController
  /** Set when the person removed the file while its batch was sending. */
  cancelled?: boolean
}

const queues = reactive<Record<string, PendingFile[]>>({})
const MAX_FILES = 10
const sending = new Set<string>()

/** True while any conversation has files attached or uploading; a reload would lose them. */
export const hasQueuedUploads = () => Object.values(queues).some((list) => list.length > 0)

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
 * Files waiting to be sent in each conversation. One batch becomes one message, with every
 * uploaded URL in its text so other IRC clients can open the same files.
 */
export function useUploads() {
  const chat = useChatStore()
  const { t, n, locale } = useI18n()
  const capability = computed(() => chat.data?.capabilities?.upload)
  const pending = computed(() => queues[chat.activeId] ?? [])

  async function add(files: FileList | File[]) {
    const limits = capability.value
    if (!limits) return chat.notify(t('mediaUi.uploads.serverUnavailable'))
    // Read back through the reactive map so pushes are tracked (`??=` returns the raw array).
    if (!queues[chat.activeId]) queues[chat.activeId] = []
    const list = queues[chat.activeId]!
    for (const file of [...files]) {
      if (list.length >= MAX_FILES)
        return chat.notify(t('mediaUi.uploads.tooManyFiles', { count: n(MAX_FILES) }, MAX_FILES))
      if (!accepts(limits.types, file.type)) {
        chat.notify(t('mediaUi.uploads.unsupportedType', { name: file.name }))
        continue
      }
      if (file.size > limits.maxBytes) {
        chat.notify(
          t('mediaUi.uploads.fileTooLarge', {
            name: file.name,
            size: formatBytes(limits.maxBytes, locale.value),
          }),
        )
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
    if (item) {
      // Removing a file that is uploading cancels it; the batch sends without it.
      item.cancelled = true
      item.controller?.abort()
      // A file that already uploaded but was never sent would stay stored on the server.
      if (item.upload && item.status !== 'done') void chat.deleteUpload(item.upload.id)
    }
    if (list) queues[conversationId] = list.filter((p) => p.key !== key)
  }

  function upload(item: PendingFile, networkId: string): Promise<Upload> {
    item.controller = new AbortController()
    return apiClient().upload(
      item.file,
      networkId,
      (progress) => {
        item.progress = progress
      },
      item.controller.signal,
    )
  }

  /** Upload all files before publishing the batch; a retry reuses already completed uploads. */
  async function sendAll(replyTo?: Message) {
    const conversationId = chat.activeId
    if (sending.has(conversationId)) return
    const batch = [...(queues[conversationId] ?? [])].filter((item) => item.status !== 'done')
    if (!batch.length) return
    sending.add(conversationId)
    // Lock the complete batch while uploading, including items waiting their turn.
    for (const item of batch) {
      item.status = 'uploading'
      item.error = item.errorKey = undefined
    }
    try {
      const networkId = chat.data?.conversations.find((c) => c.id === conversationId)?.networkId
      if (!networkId) {
        for (const item of batch) {
          item.status = 'failed'
          item.errorKey = 'mediaUi.uploads.conversationUnavailable'
        }
        return
      }
      for (const item of batch) {
        if (item.cancelled) continue
        try {
          item.upload ??= await upload(item, networkId)
        } catch (error) {
          if (item.cancelled) continue
          item.status = 'failed'
          if (error instanceof UploadError) item.errorKey = `mediaUi.uploads.${error.reason}`
          else item.error = errorText(error, (key) => t(key))
        }
      }
      // Files removed during the upload are left out of the message.
      const files = batch.filter((item) => !item.cancelled)
      if (files.some((item) => item.status === 'failed')) {
        const origin = chat.data?.conversations.find((c) => c.id === conversationId)
        if (chat.activeId !== conversationId && origin)
          chat.notify(
            t('mediaUi.uploads.failedElsewhere', {
              name:
                origin.kind === 'channel'
                  ? channelTarget(origin)
                  : (chat.directPartner(origin)?.nick ?? origin.name),
            }),
            { error: true },
          )
        return
      }
      if (!files.length) return
      const uploads = files.map((item) => ({
        ...item.upload!,
        url: new URL(item.upload!.url, location.origin).href,
      }))
      // Send to the conversation the files were attached in, even if another one is open now.
      if (
        !chat.send(uploads.map((item) => item.url).join(' '), replyTo, undefined, {
          uploads,
          conversationId,
        })
      )
        return
      for (const item of files) {
        item.status = 'done'
        remove(item.key, conversationId)
      }
    } finally {
      for (const item of batch) if (item.status === 'uploading') item.status = 'ready'
      sending.delete(conversationId)
    }
  }

  return { capability, pending, add, remove, sendAll }
}
