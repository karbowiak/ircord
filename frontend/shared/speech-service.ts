// Read-aloud's speech service (#201): an OpenAI-compatible text-to-speech endpoint that the
// browser calls directly. Its address, model and voice follow the account; the API key stays on
// each device and never reaches IRCord. Shared by the browser, the preferences sanitizer and the
// page's Content-Security-Policy, which allows exactly this service's origin for its own user.

export interface SpeechService {
  /** The API base, such as https://api.openai.com/v1; requests go to its /audio/speech. */
  baseUrl: string
  model: string
  voice: string
}
/** Where each device keeps its API key and read-aloud switch; both cleared on sign-out. */
export const speechKeyStorage = 'ircord.device.speechKey'
export const readAloudStorage = 'ircord.device.readAloud'
export const emptySpeechService: SpeechService = { baseUrl: '', model: '', voice: '' }
export const speechLimits = { baseUrl: 500, name: 100, key: 500 } as const
/** Longest text sent in one request, in characters. */
export const maxSpeechInput = 1000
/** Largest audio accepted back. */
export const maxSpeechAudio = 4 << 20

/**
 * This computer, where plain http is allowed. Anything else needs https: an https page can't
 * call http on the network (mixed content), so a LAN speech server sits behind TLS, such as a
 * reverse proxy.
 */
export function loopbackHost(host: string) {
  const name = host.toLowerCase().replace(/^\[|\]$/g, '')
  return (
    name === 'localhost' ||
    name.endsWith('.localhost') ||
    /^127\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(name) ||
    name === '::1'
  )
}

/**
 * The service's origin (scheme, host and port only) when its base URL is acceptable, else ''.
 * Acceptable: https, or http to this computer (loopback) only; no login, query or fragment, not
 * even an empty one; a host of plain letters, digits, dots and dashes (or a bracketed IPv6
 * address). The result is placed in
 * a Content-Security-Policy, so nothing that could form another token ever passes.
 */
export function speechOrigin(baseUrl: string): string {
  if (typeof baseUrl !== 'string' || !baseUrl || baseUrl.length > speechLimits.baseUrl) return ''
  if (/[\s;,'"\\*?#]/.test(baseUrl)) return ''
  let url: URL
  try {
    url = new URL(baseUrl)
  } catch {
    return ''
  }
  if (url.username || url.password || url.search || url.hash) return ''
  if (url.protocol !== 'https:' && !(url.protocol === 'http:' && loopbackHost(url.hostname))) return ''
  const origin = url.origin
  return /^https?:\/\/([a-z0-9-]+(\.[a-z0-9-]+)*|\[[0-9a-f:.]+\])(:\d{1,5})?$/.test(origin) ? origin : ''
}

const shortName = (value: unknown) =>
  typeof value === 'string' &&
  value.trim().length <= speechLimits.name &&
  !/[\u0000-\u001f\u007f]/.test(value)
    ? value.trim()
    : ''

/** Keeps a service only when its address is acceptable; otherwise it is cleared. */
export function sanitizeSpeechService(input: unknown): SpeechService {
  const v = (input && typeof input === 'object' ? input : {}) as Record<string, unknown>
  const baseUrl = typeof v.baseUrl === 'string' ? v.baseUrl.trim().replace(/\/+$/, '') : ''
  if (!speechOrigin(baseUrl)) return { ...emptySpeechService }
  return { baseUrl, model: shortName(v.model), voice: shortName(v.voice) }
}

export const speechConfigured = (service: SpeechService) =>
  !!speechOrigin(service.baseUrl) && !!service.model && !!service.voice

/** A key as typed: trimmed, no spaces or control characters, bounded. Invalid keys are ''. */
export function sanitizeSpeechKey(value: unknown) {
  const key = typeof value === 'string' ? value.trim() : ''
  return key.length <= speechLimits.key && !/[\s\u0000-\u001f\u007f]/.test(key) ? key : ''
}

/** The request the browser sends: POST <baseUrl>/audio/speech, the key only as a bearer token. */
export function speechRequest(service: SpeechService, key: string, text: string) {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (key) headers.Authorization = `Bearer ${key}`
  return {
    url: `${service.baseUrl}/audio/speech`,
    init: {
      method: 'POST',
      headers,
      body: JSON.stringify({
        model: service.model,
        voice: service.voice,
        input: text.slice(0, maxSpeechInput),
        response_format: 'mp3',
      }),
      // No IRCord cookies, no referrer, and never follow a redirect with the key.
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
      redirect: 'error',
      cache: 'no-store',
    } satisfies RequestInit,
  }
}

/**
 * The user-facing reason a request failed: an HTTP status, or a fetch that never got an answer.
 * A service on this computer is usually blocked by the browser's local network permission; a
 * service elsewhere, by CORS.
 */
export function speechFailure(error: unknown, status?: number, baseUrl = '') {
  if (status === 401 || status === 403) return `the speech service refused the API key (HTTP ${status})`
  if (status === 404) return 'the speech service has no /audio/speech at that URL (HTTP 404)'
  if (status) return `the speech service answered HTTP ${status}`
  if ((error as Error)?.name === 'AbortError') return 'stopped'
  let host = ''
  try {
    host = new URL(baseUrl).hostname
  } catch {
    /* No address to name. */
  }
  if (host && loopbackHost(host))
    return 'couldn’t reach the speech service on this computer. Check that it is running, and allow IRCord to access devices on your local network if the browser asks (or in the site settings). It must also allow requests from browsers (CORS)'
  return 'couldn’t reach the speech service. If the address is right, it doesn’t allow requests from browsers (CORS): it must answer this site’s origin and allow the Authorization header'
}
