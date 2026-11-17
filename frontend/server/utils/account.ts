import { getCookie, type H3Event } from 'h3'
import { goApiBase, backendAccount, backendDocument, changeBackendDocument } from './backend'
import { emptyLibrary, sanitizeLibrary, type PickerLibrary } from '../../shared/library'
import { sanitizeAppearance, type StoredAppearance } from '../../shared/appearance'
import { sanitizePreferences, type StoredPreferences } from '../../shared/preferences'
import {
  byteLength,
  emptyThemeLibrary,
  sanitizeThemeLibrary,
  themeLimits,
  type ThemeLibrary,
} from '../../shared/themes'
import { speechOrigin } from '../../shared/speech-service'

/**
 * Frontend-owned data (appearance, picker library, preferences, themes) is keyed by account. The mock
 * resolves the account from its own key login (server/utils/auth.ts). With the Go service,
 * call its `GET /api/v1/auth/session` with the browser's cookie here; nothing else changes.
 */
export async function accountId(event: H3Event) {
  return (goApiBase(event) ? await backendAccount(event) : await requireAccount(event)).id
}

// Nitro's `data` mount is durable on disk (.data/kv) in development and node deployments.
const storage = () => useStorage('data')
const key = (account: string, name: string) => `accounts:${account}:${name}`

export async function readAppearance(account: string, event?: H3Event): Promise<StoredAppearance> {
  const stored =
    event && goApiBase(event)
      ? (await backendDocument(event, 'appearance')).data
      : await storage().getItem<StoredAppearance>(key(account, 'appearance'))
  return stored && typeof stored === 'object' && 'settings' in stored && stored.settings
    ? {
        settings: sanitizeAppearance(stored.settings),
        updatedAt:
          'updatedAt' in stored && typeof stored.updatedAt === 'string' ? stored.updatedAt : null,
      }
    : { settings: null, updatedAt: null }
}
export async function writeAppearance(account: string, settings: unknown, event?: H3Event) {
  const value = { settings: sanitizeAppearance(settings), updatedAt: new Date().toISOString() }
  if (event && goApiBase(event))
    return changeBackendDocument<StoredAppearance>(event, 'appearance', () => value)
  await storage().setItem(key(account, 'appearance'), value)
  return value
}

export async function readLibrary(account: string, event?: H3Event): Promise<PickerLibrary> {
  const stored =
    event && goApiBase(event)
      ? (await backendDocument(event, 'library')).data
      : await storage().getItem(key(account, 'library'))
  return stored ? sanitizeLibrary(stored) : emptyLibrary()
}

// Serialise read-modify-write per account so concurrent requests from two tabs do not drop edits.
const queues = new Map<string, Promise<unknown>>()
export function updateLibrary(
  account: string,
  change: (library: PickerLibrary) => void,
  event?: H3Event,
) {
  if (event && goApiBase(event))
    return changeBackendDocument<PickerLibrary>(event, 'library', (stored) => {
      const library = stored ? sanitizeLibrary(stored) : emptyLibrary()
      change(library)
      return sanitizeLibrary({ ...library, updatedAt: new Date().toISOString() })
    })
  const previous = queues.get(account) ?? Promise.resolve()
  const next = previous
    .catch(() => {})
    .then(async () => {
      const library = await readLibrary(account)
      change(library)
      const saved = sanitizeLibrary({ ...library, updatedAt: new Date().toISOString() })
      await storage().setItem(key(account, 'library'), saved)
      return saved
    })
  queues.set(account, next)
  void next.finally(() => queues.get(account) === next && queues.delete(account))
  return next
}

export async function readThemes(account: string, event?: H3Event): Promise<ThemeLibrary> {
  const stored =
    event && goApiBase(event)
      ? (await backendDocument(event, 'themes')).data
      : await storage().getItem(key(account, 'themes'))
  return stored ? sanitizeThemeLibrary(stored) : emptyThemeLibrary()
}

/**
 * Changes the account's theme library in one read-modify-write: compare-and-swap against the Go
 * service, or serialised per account in the demo's own storage. `change` may throw an H3 error
 * to refuse the change; nothing is written then.
 */
export function updateThemes(
  account: string,
  change: (library: ThemeLibrary) => void,
  event?: H3Event,
) {
  const next = (stored: unknown) => {
    const library = stored ? sanitizeThemeLibrary(stored) : emptyThemeLibrary()
    change(library)
    const saved = sanitizeThemeLibrary({ ...library, updatedAt: new Date().toISOString() })
    // Bytes, as the backend counts them, so the plain message wins over its generic 400.
    if (byteLength(JSON.stringify(saved)) > themeLimits.document)
      throw createError({
        statusCode: 413,
        statusMessage: 'Your themes are too large to save. Remove a theme or shorten its CSS.',
      })
    return saved
  }
  if (event && goApiBase(event)) return changeBackendDocument<ThemeLibrary>(event, 'themes', next)
  const queue = `${account}:themes`
  const previous = queues.get(queue) ?? Promise.resolve()
  const result = previous
    .catch(() => {})
    .then(async () => {
      const saved = next(await storage().getItem(key(account, 'themes')))
      await storage().setItem(key(account, 'themes'), saved)
      return saved
    })
  queues.set(queue, result)
  // A refused change rejects `result` for the caller; the cleanup must not reject again unheard.
  const cleanup = () => void (queues.get(queue) === result && queues.delete(queue))
  void result.then(cleanup, cleanup)
  return result
}

export function noStore(event: H3Event) {
  setHeader(event, 'Cache-Control', 'private, no-store')
}

export async function readPreferences(
  account: string,
  event?: H3Event,
): Promise<StoredPreferences> {
  const stored =
    event && goApiBase(event)
      ? (await backendDocument(event, 'preferences')).data
      : await storage().getItem<StoredPreferences>(key(account, 'preferences'))
  return stored && typeof stored === 'object' && 'preferences' in stored && stored.preferences
    ? {
        preferences: sanitizePreferences(stored.preferences),
        updatedAt:
          'updatedAt' in stored && typeof stored.updatedAt === 'string' ? stored.updatedAt : null,
      }
    : { preferences: null, updatedAt: null }
}
export async function writePreferences(account: string, preferences: unknown, event?: H3Event) {
  const value = {
    preferences: sanitizePreferences(preferences),
    updatedAt: new Date().toISOString(),
  }
  if (event && goApiBase(event))
    return changeBackendDocument<StoredPreferences>(event, 'preferences', () => value)
  await storage().setItem(key(account, 'preferences'), value)
  return value
}

/**
 * The signed-in user's own read-aloud service origin, for their page's Content-Security-Policy;
 * '' for anyone signed out, without a service, or when it can't be read within a second. With the
 * Go service the preferences request carries the page request's cookie, so it is always that
 * user's document.
 */
export async function pageSpeechOrigin(event: H3Event) {
  if (!getCookie(event, 'ircord_session')) return ''
  const lookup = (async () => {
    const account = goApiBase(event) ? '' : await accountId(event)
    const stored = await readPreferences(account, event)
    return speechOrigin(stored.preferences?.speech.baseUrl ?? '')
  })().catch(() => '')
  // A slow backend never holds the page back: without an answer, the page simply has no origin
  // and read-aloud asks for a reload.
  let timer: ReturnType<typeof setTimeout> | undefined
  const timeout = new Promise<string>((resolve) => (timer = setTimeout(() => resolve(''), 1000)))
  try {
    return await Promise.race([lookup, timeout])
  } finally {
    clearTimeout(timer)
  }
}
