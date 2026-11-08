import createClient from 'openapi-fetch'
import type { components, paths } from '../generated/api'

type Schema = components['schemas']
export type Body<P extends keyof paths, M extends keyof paths[P]> = paths[P][M] extends {
  requestBody: { content: { 'application/json': infer T } }
} ? T : never
type Result<T> = Promise<{ data?: T; error?: unknown; response: Response }>
type Transform = <T>(value: T) => T
export interface ClientOptions {
  headers?: HeadersInit
  fetch?: (request: Request) => Promise<Response>
  transform?: Transform
  onUnauthorized?: () => void
}
export class ApiError extends Error {
  readonly status: number
  readonly statusCode: number
  readonly data: { title?: string; detail?: string; statusMessage?: string }
  constructor(response: Response, problem: unknown) {
    const value = problem && typeof problem === 'object' ? problem : {}
    const data = {
      title: 'title' in value && typeof value.title === 'string' ? value.title : undefined,
      detail: 'detail' in value && typeof value.detail === 'string' ? value.detail : undefined,
      statusMessage: 'statusMessage' in value && typeof value.statusMessage === 'string' ? value.statusMessage : undefined,
    }
    super(data.detail ?? data.statusMessage ?? data.title ?? `Request failed (${response.status}).`)
    this.name = 'ApiError'; this.status = response.status; this.statusCode = response.status; this.data = data
  }
}

/** Inputs, path/query parameters and outputs are inferred from Go's OpenAPI. */
export function createIRCordClient(apiBase: string, options: ClientOptions = {}) {
  const client = createClient<paths>({ baseUrl: apiBase.replace(/\/api\/v1\/?$/, ''), credentials: 'include', headers: options.headers, fetch: options.fetch })
  const response = async <T>(pending: Result<T>) => {
    const result = await pending
    if (!result.response.ok) {
      if (result.response.status === 401 && !result.response.url.includes('/auth/')) options.onUnauthorized?.()
      throw new ApiError(result.response, result.error)
    }
    return result
  }
  const read = async <T>(pending: Result<T>): Promise<T> => {
    const result = await response(pending)
    if (result.data === undefined) throw new Error('The server returned an empty response.')
    return options.transform ? options.transform(result.data) : result.data
  }
  const empty = async (pending: Result<unknown>): Promise<void> => { await response(pending) }
  return {
    session: () => read(client.GET('/api/v1/auth/session')),
    registration: () => read(client.GET('/api/v1/auth/registration')),
    signup: (body: Schema['SignupCredentials']) => read(client.POST('/api/v1/auth/signup', { body })),
    login: (body: Schema['Credentials']) => read(client.POST('/api/v1/auth/login', { body })),
    checkUsername: (name: string) => read(client.GET('/api/v1/auth/username', { params: { query: { name } } })),
    changePassword: (body: Schema['PasswordChange']) => empty(client.POST('/api/v1/auth/password', { body })),
    logout: () => empty(client.POST('/api/v1/auth/logout')),
    deleteAccount: () => empty(client.DELETE('/api/v1/auth/account')),
    devices: () => read(client.GET('/api/v1/auth/devices')),
    signOutDevice: (id: string) => empty(client.DELETE('/api/v1/auth/devices/{id}', { params: { path: { id } } })),
    rejectInvitation: (id: string) => empty(client.DELETE('/api/v1/invitations/{id}', { params: { path: { id } } })),
    bootstrap: () => read(client.GET('/api/v1/bootstrap')),
    messages: (id: string, query: { before?: string; around?: string; limit?: number } = {}, signal?: AbortSignal) => read(client.GET('/api/v1/conversations/{id}/messages', { params: { path: { id }, query }, signal })),
    search: (q: string, conversationId?: string, signal?: AbortSignal) => read(client.GET('/api/v1/search', { params: { query: { q, conversationId } }, signal })),
    savedMessages: (query: { cursor?: string; q?: string; limit?: number } = {}, signal?: AbortSignal) => read(client.GET('/api/v1/saved-messages', { params: { query }, signal })),
    savedMessageIds: () => read(client.GET('/api/v1/saved-message-ids')),
    saveMessage: (messageId: string, conversationId: string) => read(client.PUT('/api/v1/saved-messages/{messageId}', { params: { path: { messageId } }, body: { conversationId } })),
    unsaveMessage: (messageId: string) => empty(client.DELETE('/api/v1/saved-messages/{messageId}', { params: { path: { messageId } } })),
    inbox: (query: NonNullable<paths['/api/v1/inbox']['get']['parameters']['query']> = {}, signal?: AbortSignal) => read(client.GET('/api/v1/inbox', { params: { query }, signal })),
    inboxSummary: (signal?: AbortSignal) => read(client.GET('/api/v1/inbox/summary', { signal })),
    dismissInbox: (messageId: string) => empty(client.PUT('/api/v1/inbox/{messageId}/dismissed', { params: { path: { messageId } } })),
    restoreInbox: (messageId: string) => empty(client.DELETE('/api/v1/inbox/{messageId}/dismissed', { params: { path: { messageId } } })),
    serverLog: (id: string, query: { before?: string; limit?: number } = {}) => read(client.GET('/api/v1/networks/{id}/server-log', { params: { path: { id }, query } })),
    channels: (id: string, q: string, signal?: AbortSignal) => read(client.GET('/api/v1/networks/{id}/channels', { params: { path: { id }, query: { q, limit: 50 } }, signal })),
    modeList: (id: string, mode: string) => read(client.GET('/api/v1/conversations/{id}/lists/{mode}', { params: { path: { id, mode } } })),
    channelRegistration: (id: string) => read(client.GET('/api/v1/conversations/{id}/registration', { params: { path: { id } } })),
    registerChannel: (id: string) => read(client.POST('/api/v1/conversations/{id}/registration', { params: { path: { id } } })),
    unregisterChannel: (id: string) => empty(client.DELETE('/api/v1/conversations/{id}/registration', { params: { path: { id } } })),
    channelAccess: (id: string) => read(client.GET('/api/v1/conversations/{id}/access', { params: { path: { id } } })),
    setChannelAccess: (id: string, account: string, mode: Body<'/api/v1/conversations/{id}/access/{account}', 'put'>['mode']) => read(client.PUT('/api/v1/conversations/{id}/access/{account}', { params: { path: { id, account } }, body: { mode } })),
    removeChannelAccess: (id: string, account: string) => read(client.DELETE('/api/v1/conversations/{id}/access/{account}', { params: { path: { id, account } } })),
    transferChannel: (id: string, account: string) => read(client.POST('/api/v1/conversations/{id}/transfer', { params: { path: { id } }, body: { account } })),
    createConversation: (body: Body<'/api/v1/conversations', 'post'>) => read(client.POST('/api/v1/conversations', { body })),
    updateConversation: (id: string, body: Body<'/api/v1/conversations/{id}', 'patch'>) => read(client.PATCH('/api/v1/conversations/{id}', { params: { path: { id } }, body })),
    updateChannelSettings: (id: string, body: Body<'/api/v1/conversations/{id}/channel-settings', 'put'>) => read(client.PUT('/api/v1/conversations/{id}/channel-settings', { params: { path: { id } }, body })),
    leaveConversation: (id: string, reason?: string) => empty(client.DELETE('/api/v1/conversations/{id}', { params: { path: { id }, query: { reason } } })),
    sendMessage: (id: string, body: Body<'/api/v1/conversations/{id}/messages', 'post'>) => read(client.POST('/api/v1/conversations/{id}/messages', { params: { path: { id } }, body })),
    command: (clientId: string) => read(client.GET('/api/v1/commands/{clientId}', { params: { path: { clientId } } })),
    react: (id: string, body: Body<'/api/v1/conversations/{id}/reactions', 'post'>) => read(client.POST('/api/v1/conversations/{id}/reactions', { params: { path: { id } }, body })),
    attract: (id: string) => read(client.POST('/api/v1/conversations/{id}/attract', { params: { path: { id } } })),
    redact: (id: string, messageId: string) => read(client.DELETE('/api/v1/conversations/{id}/messages/{messageId}', { params: { path: { id, messageId } } })),
    checkNetwork: (body: Body<'/api/v1/networks/check', 'post'>, signal?: AbortSignal) => read(client.POST('/api/v1/networks/check', { body, signal })),
    createNetwork: (body: Schema['NetworkConfig']) => read(client.POST('/api/v1/networks', { body })),
    updateNetwork: (id: string, body: Schema['NetworkUpdate']) => read(client.PATCH('/api/v1/networks/{id}', { params: { path: { id } }, body })),
    deleteNetwork: (id: string) => empty(client.DELETE('/api/v1/networks/{id}', { params: { path: { id } } })),
    updateSelf: (id: string, body: Schema['SelfUpdate']) => read(client.PATCH('/api/v1/networks/{id}/self', { params: { path: { id } }, body })),
    raw: (id: string, body: Body<'/api/v1/networks/{id}/commands', 'post'>) => read(client.POST('/api/v1/networks/{id}/commands', { params: { path: { id } }, body })),
    profile: (body: Schema['ProfileUpdate']) => read(client.PATCH('/api/v1/profile', { body })),
    notificationSettings: () => read(client.GET('/api/v1/notifications/settings')),
    saveNotifications: (body: Schema['Settings']) => read(client.PUT('/api/v1/notifications/settings', { body })),
    pushKey: () => read(client.GET('/api/v1/push/key')),
    subscribePush: (body: Body<'/api/v1/push/subscriptions', 'post'>) => empty(client.POST('/api/v1/push/subscriptions', { body })),
    unsubscribePush: (endpoint: string) => empty(client.DELETE('/api/v1/push/subscriptions', { body: { endpoint } })),
    testPush: () => empty(client.POST('/api/v1/push/test')),
    fileProvider: (id: string) => read(client.GET('/api/v1/networks/{id}/uploads', { params: { path: { id } } })),
    deleteUpload: (id: string) => empty(client.DELETE('/api/v1/uploads/{id}', { params: { path: { id } } })),
    upload: (file: File, networkId: string, onProgress?: (progress: number) => void) => {
      const form = new FormData(); form.append('file', file)
      return read(client.POST('/api/v1/uploads', { params: { query: { networkId } }, body: { file }, bodySerializer: () => form, fetch: onProgress ? uploadFetch(form, onProgress) : undefined }))
    },
    connection: (id: string, signal?: AbortSignal) => read(client.GET('/api/v1/networks/{id}/connection', { params: { path: { id } }, signal })),
    trace: (id: string, query: { before?: string; limit?: number } = {}) => read(client.GET('/api/v1/networks/{id}/connection/trace', { params: { path: { id }, query } })),
    preview: (id: string, url: string) => read(client.GET('/api/v1/conversations/{id}/preview', { params: { path: { id }, query: { url } } })),
    emotes: () => read(client.GET('/api/v1/emotes')),
    importEmote: (body: Schema['EmoteImport']) => read(client.POST('/api/v1/emotes/imports', { body })),
    uploadEmote: (fields: { file: Blob; still?: Blob; shortcode: string; aliases?: string; creator?: string }) => {
      const form = new FormData()
      for (const [name, value] of Object.entries(fields)) if (value !== undefined) form.append(name, value)
      return read(client.POST('/api/v1/emotes', { body: fields as never, bodySerializer: () => form }))
    },
    updateEmote: (id: string, body: Schema['EmotePatch']) => read(client.PATCH('/api/v1/emotes/{id}', { params: { path: { id } }, body })),
    deleteEmote: (id: string) => empty(client.DELETE('/api/v1/emotes/{id}', { params: { path: { id } } })),
    hashFieldScores: () => read(client.GET('/api/v1/hash-field/scores')),
    submitHashFieldScore: (body: Body<'/api/v1/hash-field/scores', 'post'>) => read(client.POST('/api/v1/hash-field/scores', { body })),
    document: (kind: 'appearance' | 'library' | 'preferences') => read(client.GET('/api/v1/app-data/{kind}', { params: { path: { kind } } })),
    saveDocument: (kind: 'appearance' | 'library' | 'preferences', body: Schema['AppDocument']) => read(client.PUT('/api/v1/app-data/{kind}', { params: { path: { kind } }, body })),
  }
}

function uploadFetch(form: FormData, progress: (value: number) => void) {
  return (request: Request): Promise<Response> => new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open(request.method, request.url); xhr.withCredentials = true; xhr.timeout = 60_000
    request.headers.forEach((value, name) => { if (name.toLowerCase() !== 'content-type') xhr.setRequestHeader(name, value) })
    const abort = () => xhr.abort()
    request.signal.addEventListener('abort', abort, { once: true })
    xhr.onloadend = () => request.signal.removeEventListener('abort', abort)
    xhr.upload.onprogress = event => { if (event.lengthComputable) progress(event.loaded / event.total) }
    xhr.onerror = () => reject(new Error('The upload was interrupted.'))
    xhr.onabort = () => reject(new DOMException('Upload cancelled.', 'AbortError'))
    xhr.ontimeout = () => reject(new Error('The upload timed out. Try again.'))
    xhr.onload = () => resolve(new Response(xhr.responseText, { status: xhr.status, headers: { 'Content-Type': xhr.getResponseHeader('Content-Type') ?? 'application/json' } }))
    if (request.signal.aborted) return reject(new DOMException('Upload cancelled.', 'AbortError'))
    xhr.send(form)
  })
}
