import type { Message } from '../../../shared/types/chat'
import { enqueueBounded, shouldReadAloud, spokenText } from '../../../shared/read-aloud'
import { channelTarget } from '../../../shared/utils/irc'
import {
  maxSpeechAudio,
  sanitizeSpeechKey,
  speechConfigured,
  speechFailure,
  speechOrigin,
  readAloudStorage,
  speechKeyStorage,
  speechRequest,
  type SpeechService,
} from '../../../shared/speech-service'

const toggleStorage = readAloudStorage
const spokenKey = 'ircord.readAloud.spoken'
const enabled = ref(readLocal(toggleStorage, (v) => v === true))
const speaking = ref(false)
const queue: string[] = []
let generation = 0
let controller: AbortController | undefined
let audio: HTMLAudioElement | undefined
let lastWarning = 0

/** Plays one clip to its end; resolves early when reading is stopped. */
type SpeechTranslate = (key: string, values?: Record<string, string>) => string
function play(blob: Blob, signal: AbortSignal, translate: SpeechTranslate) {
  return new Promise<void>((resolve, reject) => {
    const url = URL.createObjectURL(blob)
    const element = (audio = new Audio(url))
    const done = (error?: unknown) => {
      URL.revokeObjectURL(url)
      signal.removeEventListener('abort', aborted)
      if (audio === element) audio = undefined
      if (error) reject(error)
      else resolve()
    }
    const aborted = () => {
      element.pause()
      done()
    }
    signal.addEventListener('abort', aborted, { once: true })
    element.onended = () => done()
    element.onerror = () => done(new Error(translate('workspaceUi.speech.audioFailed')))
    element.play().catch(done)
  })
}

/** A failure to tell the user about, in their words. */
class SpeechError extends Error {
  constructor(
    message: string,
    readonly kind: 'setup' | 'reload' | 'service' = 'service',
  ) {
    super(message)
    this.name = 'SpeechError'
  }
}

/** This device's API key for the speech service. Never synced, never sent to IRCord. */
const speechKey = ref(readLocal(speechKeyStorage, sanitizeSpeechKey))
function setSpeechKey(value: string) {
  speechKey.value = sanitizeSpeechKey(value)
  if (speechKey.value) writeLocal(speechKeyStorage, speechKey.value)
  else
    try {
      localStorage.removeItem(speechKeyStorage)
    } catch {
      /* Nothing was kept. */
    }
}

/**
 * The speech origin this page's Content-Security-Policy allows (the server adds the account's
 * service when it renders the page). A different, newly saved address needs a reload first.
 */
const allowedOrigin = import.meta.client
  ? (document.querySelector<HTMLMetaElement>('meta[name="ircord-speech-origin"]')?.content ?? '')
  : ''
/** Whether this page can reach the service now, or needs reloading first. */
export function speechNeedsReload(service: SpeechService) {
  return !import.meta.dev && speechOrigin(service.baseUrl) !== allowedOrigin
}

async function speak(text: string, translate: SpeechTranslate) {
  const service = usePreferencesStore().preferences.speech
  if (!speechConfigured(service))
    throw new SpeechError(translate('workspaceUi.speech.setup'), 'setup')
  if (speechNeedsReload(service))
    throw new SpeechError(
      translate('workspaceUi.speech.reload', { origin: speechOrigin(service.baseUrl) }),
      'reload',
    )
  controller = new AbortController()
  const { signal } = controller
  const { url, init } = speechRequest(service, speechKey.value, text)
  let response: Response
  try {
    response = await fetch(url, { ...init, signal })
  } catch (error) {
    if ((error as Error)?.name === 'AbortError') throw error
    throw new SpeechError(speechFailure(error, undefined, service.baseUrl, translate))
  }
  if (!response.ok)
    throw new SpeechError(speechFailure(null, response.status, service.baseUrl, translate))
  if (Number(response.headers.get('content-length')) > maxSpeechAudio)
    throw new SpeechError(translate('workspaceUi.speech.tooMuchAudio'))
  const blob = await response.blob()
  if (!blob.size) throw new SpeechError(translate('workspaceUi.speech.noAudio'))
  if (blob.size > maxSpeechAudio)
    throw new SpeechError(translate('workspaceUi.speech.tooMuchAudio'))
  if (blob.type && !blob.type.startsWith('audio/') && blob.type !== 'application/octet-stream')
    throw new SpeechError(translate('workspaceUi.speech.invalidAudio'))
  if (!signal.aborted) await play(blob, signal, translate)
}

/** One warning at a time, at most once a minute, rather than one per highlight. */
function warn(error: unknown, translate: SpeechTranslate) {
  if ((error as Error)?.name === 'AbortError' || Date.now() - lastWarning < 60_000) return
  lastWarning = Date.now()
  const kind = error instanceof SpeechError ? error.kind : undefined
  useChatStore().notify(
    kind === 'setup' || kind === 'reload'
      ? (error as Error).message
      : (error as Error)?.name === 'NotAllowedError'
        ? translate('workspaceUi.speech.blocked')
        : translate('workspaceUi.speech.readFailed', {
            error: (error as Error)?.message ?? translate('workspaceUi.speech.unknownError'),
          }),
  )
}

async function drain(translate: SpeechTranslate) {
  if (speaking.value) return
  speaking.value = true
  const current = generation
  try {
    while (queue.length && current === generation) {
      try {
        await speak(queue.shift()!, translate)
      } catch (error) {
        warn(error, translate)
      }
    }
  } finally {
    if (current === generation) speaking.value = false
  }
}

// The toggle is per device: a change in another tab of this browser applies here too.
if (import.meta.client)
  window.addEventListener('storage', (event) => {
    if (event.key === speechKeyStorage) {
      let value: unknown = ''
      try {
        value = JSON.parse(event.newValue ?? '""')
      } catch {
        /* Not ours: treat as no key. */
      }
      speechKey.value = sanitizeSpeechKey(value)
    }
    if (event.key !== toggleStorage) return
    enabled.value = event.newValue === 'true'
    if (!enabled.value) stop()
  })

function stop() {
  generation++
  queue.length = 0
  controller?.abort()
  audio?.pause()
  speaking.value = false
}

/**
 * Highlights read aloud with the account's speech service (#201), called by this browser.
 * Whether to read is a choice per device, like the notification sound, and so is the API key: it
 * stays in this browser and goes only to the speech service. Messages are spoken one at a time,
 * in order, and a burst keeps only the newest few.
 */
export function useReadAloud() {
  const { t } = useI18n()
  const translate: SpeechTranslate = (key, values) => (values ? t(key, values) : t(key))
  function setEnabled(value: boolean) {
    enabled.value = value
    writeLocal(toggleStorage, value)
    if (!value) stop()
  }
  function say(text: string) {
    if (!enabled.value) return
    enqueueBounded(queue, text)
    void drain(translate)
  }
  /** Reads a sample now, whatever the toggle says; errors go to the caller. */
  async function test(text: string) {
    stop()
    speaking.value = true
    const current = generation
    try {
      await speak(text, translate)
    } finally {
      if (current === generation) speaking.value = false
      // Highlights that arrived during the sample are read now, not with the next one.
      if (queue.length) void drain(translate)
    }
  }
  return { enabled, speaking, setEnabled, say, stop, test, speechKey, setSpeechKey }
}

/** Claims a message for this browser, so two open tabs don't both read it. */
function claimNow(id: string) {
  const spoken = readLocal(spokenKey, (v) =>
    Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : [],
  )
  if (spoken.includes(id)) return false
  writeLocal(spokenKey, [...spoken.slice(-49), id])
  return true
}
/** The check and the write run under a lock shared by this site's tabs, so only one wins. */
async function claim(id: string) {
  if (!navigator.locks) return claimNow(id)
  return navigator.locks.request('ircord.readAloud.claim', () => claimNow(id))
}

/** Reads incoming highlights while the toggle is on. Call once from the workspace. */
export function useHighlightReader() {
  const { t } = useI18n()
  const chat = useChatStore()
  const notifications = useNotificationStore()
  const reader = useReadAloud()
  const off = chat.onIncoming((message: Message) => {
    if (!reader.enabled.value) return
    const c = chat.data?.conversations.find((x) => x.id === message.conversationId)
    if (!c) return
    const sender = chat.member(message.senderId)
    const you = chat.recipientFor(c, message)
    if (!shouldReadAloud(notifications.settings, c, message, you, sender?.nick, new Date(), sender))
      return
    const text = spokenText(
      {
        sender: sender?.nick,
        channel: c.kind === 'channel' && !c.party ? channelTarget(c) : undefined,
        text: message.text,
        gif: !!message.gif,
      },
      (key, values) => (values ? t(key, values) : t(key)),
    )
    void claim(message.id).then((mine) => {
      if (mine) reader.say(text)
    })
  })
  // Removed on another device: switch off here too, rather than stay on with nothing to call.
  const preferences = usePreferencesStore()
  watch(
    () => speechConfigured(preferences.preferences.speech),
    (now, before) => {
      if (before && !now) reader.setEnabled(false)
    },
  )
  onBeforeUnmount(() => {
    off()
    reader.stop()
  })
}
